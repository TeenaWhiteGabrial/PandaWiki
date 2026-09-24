import ErrorJSON from '@/assets/json/error.json';
import Card from '@/components/Card';
import { ModelProvider } from '@/constant/enums';
import {
  postApiV1ModelSwitchMode,
  putApiV1Model,
  getApiV1ModelModeSetting,
} from '@/request/Model';
import { GithubComChaitinPandaWikiDomainModelListItem } from '@/request/types';
import { addOpacityToColor } from '@/utils';
import { message, Modal } from '@/components/ui';
import { Box, Button, Stack, Switch, useTheme } from '@mui/material';
import LottieIcon from '../../LottieIcon';
import {
  useState,
  useEffect,
  lazy,
  Suspense,
  useRef,
  forwardRef,
  useImperativeHandle,
  useMemo,
} from 'react';
import {
  convertLocalModelToUIModel,
  modelService,
} from '@/services/modelService';
import { GithubComChaitinPandaWikiDomainModelProvider } from '@/request/types';
import {
  normalizeModelProviderForApi,
  normalizeModelProviderForDisplay,
} from '@/utils/modelProvider';
import AutoModelConfig, {
  AutoModelConfigRef,
  AutoModelConfigFormData,
} from './AutoModelConfig';

type ModelDataMap = Record<
  'chat' | 'embedding' | 'rerank' | 'analysis' | 'analysis-vl',
  GithubComChaitinPandaWikiDomainModelListItem | null
>;

const ModelModal = lazy(() =>
  import('@ctzhian/modelkit').then(
    (mod: typeof import('@ctzhian/modelkit')) => ({ default: mod.ModelModal }),
  ),
);

export interface ModelConfigRef {
  getAutoConfigFormData: () => AutoModelConfigFormData | null;
  handleClose: () => void;
  onSubmit: () => Promise<void>;
}

interface ModelConfigProps {
  onCloseModal: () => void;
  chatModelData: GithubComChaitinPandaWikiDomainModelListItem | null;
  embeddingModelData: GithubComChaitinPandaWikiDomainModelListItem | null;
  rerankModelData: GithubComChaitinPandaWikiDomainModelListItem | null;
  analysisModelData: GithubComChaitinPandaWikiDomainModelListItem | null;
  analysisVLModelData: GithubComChaitinPandaWikiDomainModelListItem | null;
  getModelList: () => void;
  autoSwitchToAutoMode?: boolean;
  hideDocumentationHint?: boolean;
  showTip?: boolean;
  showSaveBtn?: boolean;
}

const ModelConfig = forwardRef<ModelConfigRef, ModelConfigProps>(
  (props, ref) => {
    const theme = useTheme();
    const {
      onCloseModal,
      chatModelData,
      embeddingModelData,
      rerankModelData,
      analysisModelData,
      analysisVLModelData,
      getModelList,
      autoSwitchToAutoMode = false,
      hideDocumentationHint = false,
      showTip = false,
      showSaveBtn = true,
    } = props;

    const [autoConfigMode, setAutoConfigMode] = useState(true);
    const [hasAutoSwitched, setHasAutoSwitched] = useState(false);
    const [tempMode, setTempMode] = useState<'auto' | 'manual'>('auto');
    const [savedMode, setSavedMode] = useState<'auto' | 'manual'>('auto');
    const [isSaving, setIsSaving] = useState(false);
    const [initialAutoProvider, setInitialAutoProvider] =
      useState<AutoModelConfigFormData | null>(null);
    const [hasConfigChanged, setHasConfigChanged] = useState(false);

    const [modelData, setModelData] = useState<ModelDataMap>({
      chat: chatModelData,
      embedding: embeddingModelData,
      rerank: rerankModelData,
      analysis: analysisModelData,
      'analysis-vl': analysisVLModelData,
    });

    const cacheModelData = useRef<Record<string, unknown>>({});

    const autoConfigRef = useRef<AutoModelConfigRef>(null);

    const [addOpen, setAddOpen] = useState(false);
    const [addType, setAddType] = useState<
      'chat' | 'embedding' | 'rerank' | 'analysis' | 'analysis-vl'
    >('chat');
    const [openingAdd, setOpeningAdd] = useState<
      'chat' | 'embedding' | 'rerank' | 'analysis' | 'analysis-vl' | null
    >(null);

    const handleOpenAdd = async (
      type: 'chat' | 'embedding' | 'rerank' | 'analysis' | 'analysis-vl',
    ) => {
      try {
        setOpeningAdd(type);
        // 预加载 modal 代码分块，避免首次打开白屏
        await import('@ctzhian/modelkit');
        setAddType(type);
        setAddOpen(true);
      } finally {
        setOpeningAdd(null);
      }
    };

    // 组件挂载时,获取当前配置
    useEffect(() => {
      const fetchModeSetting = async () => {
        try {
          const setting = await getApiV1ModelModeSetting();
          if (setting) {
            setAutoConfigMode(true);
            setTempMode('auto');
            setSavedMode('auto');

            setInitialAutoProvider(
              (setting.auto_provider as AutoModelConfigFormData | null) || null,
            );
          }
        } catch (err) {
          console.error('获取模型配置失败:', err);
        }
      };
      fetchModeSetting();
    }, []);

    // 如果需要自动切换到自动配置模式
    useEffect(() => {
      const switchToAutoMode = async () => {
        if (autoSwitchToAutoMode && !hasAutoSwitched) {
          try {
            const currentSetting = await getApiV1ModelModeSetting();
            await postApiV1ModelSwitchMode({
              mode: 'auto',
              auto_provider: currentSetting.auto_provider,
            });
            setAutoConfigMode(true);
            setTempMode('auto');
            setSavedMode('auto');
            setHasAutoSwitched(true);
            getModelList();
          } catch (err) {
            console.error('切换到自动配置模式失败:', err);
          }
        }
      };
      switchToAutoMode();
    }, [autoSwitchToAutoMode, hasAutoSwitched, getModelList]);

    // 处理关闭弹窗
    const handleCloseModal = () => {
      // 判断是否有未应用的更改
      const hasUnappliedChanges = tempMode !== savedMode || hasConfigChanged;

      if (hasUnappliedChanges) {
        Modal.confirm({
          title: '提示',
          content: '有未应用的设置，是否确认关闭？',
          onOk: () => {
            onCloseModal();
          },
          okText: '确认',
          cancelText: '取消',
        });
      } else {
        onCloseModal();
      }
    };

    // 暴露方法给父组件
    useImperativeHandle(ref, () => ({
      getAutoConfigFormData: () => {
        if (autoConfigMode && autoConfigRef.current) {
          return autoConfigRef.current.getFormData();
        }
        return null;
      },
      onSubmit: handleSave,
      handleClose: handleCloseModal,
    }));

    useEffect(() => {
      setModelData({
        chat: chatModelData,
        embedding: embeddingModelData,
        rerank: rerankModelData,
        analysis: analysisModelData,
        'analysis-vl': analysisVLModelData,
      });
    }, [
      chatModelData,
      embeddingModelData,
      rerankModelData,
      analysisModelData,
      analysisVLModelData,
    ]);

    const handleSave = async () => {
      if (!showSaveBtn) {
        return await performSave();
      }

      if (tempMode !== savedMode || hasConfigChanged) {
        // 检测是否切换了模式
        const isModeChanged = tempMode !== savedMode;
        // 检测向量模型是否变更 (比较 provider + model 组合)
        const isEmbeddingModelChanged = !!cacheModelData.current['embedding'];

        // 如果切换了模式或修改了向量模型,需要确认
        if (isModeChanged || isEmbeddingModelChanged) {
          Modal.confirm({
            title: '确认操作',
            content: '此操作会在后台重建问答索引，请确认是否继续？',
            onOk: async () => {
              await performSave();
            },
            okText: '确认',
            cancelText: '取消',
          });
        } else {
          await performSave();
        }
      }
    };

    const performSave = async () => {
      setIsSaving(true);
      const modelConfigList = Object.keys(cacheModelData.current);

      try {
        const requestData: {
          mode: 'auto' | 'manual';
          auto_provider?: {
            provider: GithubComChaitinPandaWikiDomainModelProvider;
            base_url: string;
            base_urls?: AutoModelConfigFormData['base_urls'];
            api_key: string;
            models: AutoModelConfigFormData['models'];
          };
        } = {
          mode: 'auto',
        };

        // 当前管理端只开放自动模式，保存时始终读取自动供应商配置
        if (autoConfigRef.current) {
          const formData = autoConfigRef.current.getFormData();
          if (formData) {
            if (
              !formData.provider ||
              !formData.base_url ||
              !formData.models.chat ||
              !formData.models.embedding ||
              !formData.models.rerank ||
              !formData.models.analysis ||
              !formData.models.analysis_vl
            ) {
              message.error('请完成自动模式的供应商和模型配置');
              return;
            }
            requestData.auto_provider = {
              provider: normalizeModelProviderForApi(formData.provider),
              base_url: formData.base_url,
              base_urls: formData.base_urls,
              api_key: formData.api_key,
              models: formData.models,
            };
          }
        }

        await postApiV1ModelSwitchMode(requestData);
        setTempMode('auto');
        setSavedMode('auto');
        setAutoConfigMode(true);
        setHasConfigChanged(false); // 重置变更标记

        // 更新保存的初始值
        if (autoConfigRef.current) {
          const formData = autoConfigRef.current.getFormData();
          if (formData) {
            setInitialAutoProvider({
              ...formData,
              provider: normalizeModelProviderForDisplay(formData.provider),
            });
          }
        }

        if (showSaveBtn && modelConfigList.length === 0) {
          message.success('已保存自动配置');
        }
        cacheModelData.current = {};
        await getModelList(); // 刷新模型列表
      } finally {
        setIsSaving(false);
      }
    };

    const IconModel = modelData.chat
      ? ModelProvider[modelData.chat.provider as keyof typeof ModelProvider]
          .icon
      : null;

    const IconEmbeddingModel = modelData.embedding
      ? ModelProvider[
          modelData.embedding.provider as keyof typeof ModelProvider
        ].icon
      : null;

    const IconRerankModel = modelData.rerank
      ? ModelProvider[modelData.rerank.provider as keyof typeof ModelProvider]
          .icon
      : null;

    const IconAnalysisModel = modelData.analysis
      ? ModelProvider[modelData.analysis.provider as keyof typeof ModelProvider]
          .icon
      : null;

    const IconAnalysisVLModel = modelData['analysis-vl']
      ? ModelProvider[
          modelData['analysis-vl'].provider as keyof typeof ModelProvider
        ].icon
      : null;

    const modelModalChatData = useMemo(() => {
      return convertLocalModelToUIModel(modelData.chat);
    }, [modelData.chat]);

    const modelModalEmbeddingData = useMemo(() => {
      return convertLocalModelToUIModel(modelData.embedding);
    }, [modelData.embedding]);

    const modelModalRerankData = useMemo(() => {
      return convertLocalModelToUIModel(modelData.rerank);
    }, [modelData.rerank]);

    const modelModalAnalysisData = useMemo(() => {
      return convertLocalModelToUIModel(modelData.analysis);
    }, [modelData.analysis]);

    const analysisVLModel = modelData['analysis-vl'];
    const modelModalAnalysisVLData = useMemo(() => {
      return convertLocalModelToUIModel(analysisVLModel);
    }, [analysisVLModel]);

    return (
      <Stack gap={0}>
        <Box
          sx={{
            pl: 2,
            display: 'flex',
            alignItems: 'flex-start',
          }}
        >
          <Box sx={{ flex: 1 }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                fontSize: 14,
                fontWeight: 'bold',
                color: 'text.primary',
                mb: '16px',
              }}
            >
              <Box
                sx={{
                  width: 4,
                  height: 10,
                  bgcolor: 'primary.main',
                  borderRadius: '30%',
                  mr: 1,
                }}
              />
              模型配置
            </Box>
            <Stack gap={1} direction='row' alignItems='center'>
              <Box
                sx={{
                  fontSize: 13,
                  color: 'text.secondary',
                  lineHeight: '24px',
                }}
              >
                当前使用自动配置模式：统一配置供应商、Base URL 和 API
                Key，并分别选择对话、向量、重排序和文档分析模型。
              </Box>
              {showSaveBtn && (
                <Box
                  sx={{
                    fontSize: 12,
                    color: 'text.tertiary',
                    ml: 2,
                    display: 'flex',
                    alignItems: 'center',
                    gap: 0.5,
                  }}
                >
                  <Box
                    component='span'
                    sx={{
                      color: 'warning.main',
                      fontWeight: 'bold',
                    }}
                  >
                    提示：
                  </Box>
                  修改向量模型会在后台重建问答索引
                </Box>
              )}
            </Stack>
          </Box>
          {(tempMode !== savedMode || hasConfigChanged) && showSaveBtn && (
            <Button
              variant='contained'
              size='small'
              loading={isSaving}
              onClick={handleSave}
              sx={{ mt: 3 }}
            >
              应用
            </Button>
          )}
        </Box>
        {autoConfigMode ? (
          <AutoModelConfig
            ref={autoConfigRef}
            showTip={showTip}
            initialValue={initialAutoProvider}
            onDataChange={() => setHasConfigChanged(true)}
          />
        ) : (
          <>
            {/* Chat */}
            <Card
              sx={{
                flex: 1,
                p: 2,
                overflow: 'hidden',
                overflowY: 'auto',
                border: '1px solid',
                borderColor: 'divider',
              }}
            >
              <Stack
                direction={'row'}
                alignItems={'center'}
                justifyContent={'space-between'}
              >
                <Box>
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    sx={{ width: 500 }}
                  >
                    {modelData.chat ? (
                      <>
                        {IconModel && <IconModel sx={{ fontSize: 18 }} />}
                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            color: 'text.tertiary',
                          }}
                        >
                          {ModelProvider[
                            modelData.chat
                              .provider as keyof typeof ModelProvider
                          ].cn ||
                            ModelProvider[
                              modelData.chat
                                .provider as keyof typeof ModelProvider
                            ].label ||
                            '其他'}
                          &nbsp;&nbsp;/
                        </Box>
                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            fontFamily: 'AlibabaPuHuiTi',
                            ml: -0.5,
                          }}
                        >
                          {modelData.chat.model}
                        </Box>
                        <Box
                          sx={{
                            fontSize: 12,
                            px: 1,
                            lineHeight: '20px',
                            borderRadius: '10px',
                            bgcolor: addOpacityToColor(
                              theme.palette.primary.main,
                              0.1,
                            ),
                            color: 'primary.main',
                          }}
                        >
                          智能对话模型
                        </Box>
                      </>
                    ) : (
                      <Box
                        sx={{
                          fontSize: 14,
                          lineHeight: '20px',
                          fontFamily: 'AlibabaPuHuiTi',
                          ml: -0.5,
                        }}
                      >
                        智能对话模型
                      </Box>
                    )}
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                        color: 'primary.main',
                      }}
                    >
                      大模型
                    </Box>
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                        color: 'primary.main',
                      }}
                    >
                      必选
                    </Box>
                  </Stack>
                  <Box sx={{ fontSize: 12, color: 'text.tertiary', mt: 1 }}>
                    在
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      智能问答{' '}
                    </Box>
                    和
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      摘要生成{' '}
                    </Box>
                    过程中使用。
                  </Box>
                </Box>
                <Box sx={{ flexGrow: 1, flexSelf: 'flex-start' }}>
                  {modelData.chat ? (
                    <Box
                      sx={{
                        display: 'inline-block',
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.success.main,
                          0.1,
                        ),
                        color: 'success.main',
                      }}
                    >
                      状态正常
                    </Box>
                  ) : (
                    <Stack direction={'row'} alignItems={'center'} gap={1}>
                      <Box
                        sx={{
                          fontSize: 12,
                          px: 1,
                          lineHeight: '20px',
                          borderRadius: '10px',
                          bgcolor: addOpacityToColor(
                            theme.palette.error.main,
                            0.1,
                          ),
                          color: 'error.main',
                          whiteSpace: 'nowrap',
                          flexShrink: 0,
                        }}
                      >
                        必填配置
                      </Box>
                      {!hideDocumentationHint && (
                        <>
                          <Stack
                            alignItems={'center'}
                            justifyContent={'center'}
                            sx={{ width: 22, height: 22, cursor: 'pointer' }}
                          >
                            <LottieIcon
                              id='warning'
                              src={ErrorJSON}
                              style={{ width: 20, height: 20 }}
                            />
                          </Stack>
                          <Box sx={{ color: 'error.main', fontSize: 12 }}>
                            未配置无法使用，请先完成当前类型的模型配置。
                          </Box>
                        </>
                      )}
                    </Stack>
                  )}
                </Box>
                <Button
                  size='small'
                  variant='outlined'
                  loading={openingAdd === 'chat'}
                  onClick={() => handleOpenAdd('chat')}
                >
                  {modelData.chat ? '修改' : '配置'}
                </Button>
              </Stack>
            </Card>

            {/* Embedding */}
            <Card
              sx={{
                flex: 1,
                p: 2,
                overflow: 'hidden',
                overflowY: 'auto',
                border: '1px solid',
                borderColor: 'divider',
              }}
            >
              <Stack
                direction={'row'}
                alignItems={'center'}
                justifyContent={'space-between'}
              >
                <Box>
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    sx={{ width: 500 }}
                  >
                    {modelData.embedding ? (
                      <>
                        {IconEmbeddingModel && (
                          <IconEmbeddingModel sx={{ fontSize: 18 }} />
                        )}

                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            color: 'text.tertiary',
                          }}
                        >
                          {ModelProvider[
                            modelData.embedding
                              .provider as keyof typeof ModelProvider
                          ].cn ||
                            ModelProvider[
                              modelData.embedding
                                .provider as keyof typeof ModelProvider
                            ].label ||
                            '其他'}
                          &nbsp;&nbsp;/
                        </Box>
                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            fontFamily: 'AlibabaPuHuiTi',
                            ml: -0.5,
                          }}
                        >
                          {modelData.embedding.model}
                        </Box>
                        <Box
                          sx={{
                            fontSize: 12,
                            px: 1,
                            lineHeight: '20px',
                            borderRadius: '10px',
                            bgcolor: addOpacityToColor(
                              theme.palette.primary.main,
                              0.1,
                            ),
                            color: 'primary.main',
                          }}
                        >
                          向量模型
                        </Box>
                      </>
                    ) : (
                      <Box
                        sx={{
                          fontSize: 14,
                          lineHeight: '20px',
                          fontFamily: 'AlibabaPuHuiTi',
                          ml: -0.5,
                        }}
                      >
                        向量模型
                      </Box>
                    )}
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                        color: 'primary.main',
                      }}
                    >
                      小模型
                    </Box>
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                        color: 'primary.main',
                      }}
                    >
                      必选
                    </Box>
                  </Stack>
                  <Box sx={{ fontSize: 12, color: 'text.tertiary', mt: 1 }}>
                    在
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      内容发布{' '}
                    </Box>
                    和
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      智能问答{' '}
                    </Box>
                    和
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      智能搜索{' '}
                    </Box>
                    过程中使用。
                  </Box>
                </Box>
                <Box sx={{ flexGrow: 1, flexSelf: 'flex-start' }}>
                  {modelData.embedding ? (
                    <Box
                      sx={{
                        display: 'inline-block',
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.success.main,
                          0.1,
                        ),
                        color: 'success.main',
                      }}
                    >
                      状态正常
                    </Box>
                  ) : (
                    <Stack direction={'row'} alignItems={'center'} gap={1}>
                      <Box
                        sx={{
                          fontSize: 12,
                          px: 1,
                          lineHeight: '20px',
                          borderRadius: '10px',
                          bgcolor: addOpacityToColor(
                            theme.palette.error.main,
                            0.1,
                          ),
                          color: 'error.main',
                          whiteSpace: 'nowrap',
                          flexShrink: 0,
                        }}
                      >
                        必填配置
                      </Box>
                      {!hideDocumentationHint && (
                        <>
                          <Stack
                            alignItems={'center'}
                            justifyContent={'center'}
                            sx={{ width: 22, height: 22, cursor: 'pointer' }}
                          >
                            <LottieIcon
                              id='warning'
                              src={ErrorJSON}
                              style={{ width: 20, height: 20 }}
                            />
                          </Stack>
                          <Box sx={{ color: 'error.main', fontSize: 12 }}>
                            未配置无法使用，请先完成当前类型的模型配置。
                          </Box>
                        </>
                      )}
                    </Stack>
                  )}
                </Box>
                <Button
                  size='small'
                  variant='outlined'
                  loading={openingAdd === 'embedding'}
                  onClick={() => handleOpenAdd('embedding')}
                >
                  {modelData.embedding ? '修改' : '配置'}
                </Button>
              </Stack>
            </Card>

            {/* Rerank */}
            <Card
              sx={{
                flex: 1,
                p: 2,
                overflow: 'hidden',
                overflowY: 'auto',
                border: '1px solid',
                borderColor: 'divider',
              }}
            >
              <Stack
                direction={'row'}
                alignItems={'center'}
                justifyContent={'space-between'}
              >
                <Box>
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    sx={{ width: 500 }}
                  >
                    {modelData.rerank ? (
                      <>
                        {IconRerankModel && (
                          <IconRerankModel sx={{ fontSize: 18 }} />
                        )}

                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            color: 'text.tertiary',
                          }}
                        >
                          {ModelProvider[
                            modelData.rerank
                              .provider as keyof typeof ModelProvider
                          ].cn ||
                            ModelProvider[
                              modelData.rerank
                                .provider as keyof typeof ModelProvider
                            ].label ||
                            '其他'}
                          &nbsp;&nbsp;/
                        </Box>
                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            fontFamily: 'AlibabaPuHuiTi',
                            ml: -0.5,
                          }}
                        >
                          {modelData.rerank.model}
                        </Box>
                        <Box
                          sx={{
                            fontSize: 12,
                            px: 1,
                            lineHeight: '20px',
                            borderRadius: '10px',
                            bgcolor: addOpacityToColor(
                              theme.palette.primary.main,
                              0.1,
                            ),
                            color: 'primary.main',
                          }}
                        >
                          重排序模型
                        </Box>
                      </>
                    ) : (
                      <Box
                        sx={{
                          fontSize: 14,
                          lineHeight: '20px',
                          fontFamily: 'AlibabaPuHuiTi',
                          ml: -0.5,
                        }}
                      >
                        重排序模型
                      </Box>
                    )}
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                        color: 'primary.main',
                      }}
                    >
                      小模型
                    </Box>
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                        color: 'primary.main',
                      }}
                    >
                      必选
                    </Box>
                  </Stack>
                  <Box sx={{ fontSize: 12, color: 'text.tertiary', mt: 1 }}>
                    在
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      智能问答{' '}
                    </Box>
                    和
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      智能搜索{' '}
                    </Box>
                    过程中使用。
                  </Box>
                </Box>
                <Box sx={{ flexGrow: 1, flexSelf: 'flex-start' }}>
                  {modelData.rerank ? (
                    <Box
                      sx={{
                        display: 'inline-block',
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.success.main,
                          0.1,
                        ),
                        color: 'success.main',
                      }}
                    >
                      状态正常
                    </Box>
                  ) : (
                    <Stack direction={'row'} alignItems={'center'} gap={1}>
                      <Box
                        sx={{
                          fontSize: 12,
                          px: 1,
                          lineHeight: '20px',
                          borderRadius: '10px',
                          bgcolor: addOpacityToColor(
                            theme.palette.error.main,
                            0.1,
                          ),
                          color: 'error.main',
                          whiteSpace: 'nowrap',
                          flexShrink: 0,
                        }}
                      >
                        必填配置
                      </Box>
                      {!hideDocumentationHint && (
                        <>
                          <Stack
                            alignItems={'center'}
                            justifyContent={'center'}
                            sx={{ width: 22, height: 22, cursor: 'pointer' }}
                          >
                            <LottieIcon
                              id='warning'
                              src={ErrorJSON}
                              style={{ width: 20, height: 20 }}
                            />
                          </Stack>
                          <Box sx={{ color: 'error.main', fontSize: 12 }}>
                            未配置无法使用，请先完成当前类型的模型配置。
                          </Box>
                        </>
                      )}
                    </Stack>
                  )}
                </Box>
                <Button
                  size='small'
                  variant='outlined'
                  loading={openingAdd === 'rerank'}
                  onClick={() => handleOpenAdd('rerank')}
                >
                  {modelData.rerank ? '修改' : '配置'}
                </Button>
              </Stack>
            </Card>

            {/* Analysis */}
            <Card
              sx={{
                flex: 1,
                p: 2,
                overflow: 'hidden',
                overflowY: 'auto',
                border: '1px solid',
                borderColor: 'divider',
              }}
            >
              <Stack
                direction={'row'}
                alignItems={'center'}
                justifyContent={'space-between'}
              >
                <Box>
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    sx={{ width: 500 }}
                  >
                    {modelData.analysis ? (
                      <>
                        {IconAnalysisModel && (
                          <IconAnalysisModel sx={{ fontSize: 18 }} />
                        )}

                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            color: 'text.tertiary',
                          }}
                        >
                          {ModelProvider[
                            modelData.analysis
                              .provider as keyof typeof ModelProvider
                          ].cn ||
                            ModelProvider[
                              modelData.analysis
                                .provider as keyof typeof ModelProvider
                            ].label ||
                            '其他'}
                          &nbsp;&nbsp;/
                        </Box>
                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            fontFamily: 'AlibabaPuHuiTi',
                            ml: -0.5,
                          }}
                        >
                          {modelData.analysis.model}
                        </Box>
                        <Box
                          sx={{
                            fontSize: 12,
                            px: 1,
                            lineHeight: '20px',
                            borderRadius: '10px',
                            bgcolor: addOpacityToColor(
                              theme.palette.primary.main,
                              0.1,
                            ),
                            color: 'primary.main',
                          }}
                        >
                          文档分析模型
                        </Box>
                      </>
                    ) : (
                      <Box
                        sx={{
                          fontSize: 14,
                          lineHeight: '20px',
                          fontFamily: 'AlibabaPuHuiTi',
                          ml: -0.5,
                        }}
                      >
                        文档分析模型
                      </Box>
                    )}
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                        color: 'primary.main',
                      }}
                    >
                      小模型
                    </Box>
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                        color: 'primary.main',
                      }}
                    >
                      必选
                    </Box>
                  </Stack>
                  <Box sx={{ fontSize: 12, color: 'text.tertiary', mt: 1 }}>
                    在
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      内容发布{' '}
                    </Box>
                    和
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      智能问答{' '}
                    </Box>
                    过程中使用。
                  </Box>
                </Box>
                <Box sx={{ flexGrow: 1, flexSelf: 'flex-start' }}>
                  {modelData.analysis ? (
                    <Box
                      sx={{
                        display: 'inline-block',
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.success.main,
                          0.1,
                        ),
                        color: 'success.main',
                      }}
                    >
                      状态正常
                    </Box>
                  ) : (
                    <Stack direction={'row'} alignItems={'center'} gap={1}>
                      <Box
                        sx={{
                          fontSize: 12,
                          px: 1,
                          lineHeight: '20px',
                          borderRadius: '10px',
                          bgcolor: addOpacityToColor(
                            theme.palette.error.main,
                            0.1,
                          ),
                          color: 'error.main',
                          whiteSpace: 'nowrap',
                          flexShrink: 0,
                        }}
                      >
                        必填配置
                      </Box>
                      {!hideDocumentationHint && (
                        <>
                          <Stack
                            alignItems={'center'}
                            justifyContent={'center'}
                            sx={{ width: 22, height: 22, cursor: 'pointer' }}
                          >
                            <LottieIcon
                              id='warning'
                              src={ErrorJSON}
                              style={{ width: 20, height: 20 }}
                            />
                          </Stack>
                          <Box sx={{ color: 'error.main', fontSize: 12 }}>
                            未配置无法使用，请先完成当前类型的模型配置。
                          </Box>
                        </>
                      )}
                    </Stack>
                  )}
                </Box>
                <Button
                  size='small'
                  variant='outlined'
                  loading={openingAdd === 'analysis'}
                  onClick={() => handleOpenAdd('analysis')}
                >
                  {modelData.analysis ? '修改' : '配置'}
                </Button>
              </Stack>
            </Card>

            {/* Analysis-VL */}
            <Card
              sx={{
                flex: 1,
                p: 2,
                overflow: 'hidden',
                overflowY: 'auto',
                border: '1px solid',
                borderColor: 'divider',
              }}
            >
              <Stack
                direction={'row'}
                alignItems={'center'}
                justifyContent={'space-between'}
              >
                <Box>
                  <Stack
                    direction={'row'}
                    alignItems={'center'}
                    gap={1}
                    sx={{ width: 500 }}
                  >
                    {modelData['analysis-vl'] ? (
                      <>
                        {IconAnalysisVLModel && (
                          <IconAnalysisVLModel sx={{ fontSize: 18 }} />
                        )}
                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            color: 'text.tertiary',
                          }}
                        >
                          {ModelProvider[
                            modelData['analysis-vl']
                              .provider as keyof typeof ModelProvider
                          ].cn ||
                            ModelProvider[
                              modelData['analysis-vl']
                                .provider as keyof typeof ModelProvider
                            ].label ||
                            '其他'}
                          &nbsp;&nbsp;/
                        </Box>
                        <Box
                          sx={{
                            fontSize: 14,
                            lineHeight: '20px',
                            fontFamily: 'AlibabaPuHuiTi',
                            ml: -0.5,
                          }}
                        >
                          {modelData['analysis-vl'].model}
                        </Box>
                        <Box
                          sx={{
                            fontSize: 12,
                            px: 1,
                            lineHeight: '20px',
                            borderRadius: '10px',
                            bgcolor: addOpacityToColor(
                              theme.palette.primary.main,
                              0.1,
                            ),
                            color: 'primary.main',
                          }}
                        >
                          图像分析模型
                        </Box>
                      </>
                    ) : (
                      <Box
                        sx={{
                          fontSize: 14,
                          lineHeight: '20px',
                          fontFamily: 'AlibabaPuHuiTi',
                          ml: -0.5,
                        }}
                      >
                        图像分析模型
                      </Box>
                    )}
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.primary.main,
                          0.1,
                        ),
                        color: 'primary.main',
                      }}
                    >
                      视觉模型
                    </Box>
                    <Box
                      sx={{
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: theme.palette.divider,
                        color: 'text.tertiary',
                      }}
                    >
                      可选
                    </Box>
                    {modelData['analysis-vl'] &&
                      modelData['analysis-vl']?.id && (
                        <Switch
                          size='small'
                          checked={!!modelData['analysis-vl']?.is_active}
                          onChange={() => {
                            const analysisVL = modelData['analysis-vl'];
                            if (
                              !analysisVL?.id ||
                              !analysisVL.model ||
                              !analysisVL.provider ||
                              !analysisVL.type
                            ) {
                              return;
                            }
                            putApiV1Model({
                              ...analysisVL,
                              id: analysisVL.id,
                              base_url: analysisVL.base_url || '',
                              model: analysisVL.model,
                              provider: analysisVL.provider,
                              type: analysisVL.type,
                              is_active: !analysisVL.is_active,
                            }).then(() => {
                              message.success('修改成功');
                              getModelList();
                            });
                          }}
                        />
                      )}
                  </Stack>
                  <Box sx={{ fontSize: 12, color: 'text.tertiary', mt: 1 }}>
                    在
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      内容发布{' '}
                    </Box>
                    和
                    <Box component='span' sx={{ fontWeight: 'bold' }}>
                      {' '}
                      智能问答{' '}
                    </Box>
                    过程中使用，启用后图像分析能力可用，可选配置。
                  </Box>
                </Box>
                <Box sx={{ flexGrow: 1, flexSelf: 'flex-start' }}>
                  {modelData['analysis-vl'] ? (
                    <Box
                      sx={{
                        display: 'inline-block',
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: addOpacityToColor(
                          theme.palette.success.main,
                          0.1,
                        ),
                        color: 'success.main',
                      }}
                    >
                      状态正常
                    </Box>
                  ) : (
                    <Box
                      sx={{
                        display: 'inline-block',
                        fontSize: 12,
                        px: 1,
                        lineHeight: '20px',
                        borderRadius: '10px',
                        bgcolor: theme.palette.divider,
                        color: 'text.tertiary',
                        whiteSpace: 'nowrap',
                        flexShrink: 0,
                      }}
                    >
                      可选模型
                    </Box>
                  )}
                </Box>
                <Button
                  size='small'
                  variant='outlined'
                  loading={openingAdd === 'analysis-vl'}
                  onClick={() => handleOpenAdd('analysis-vl')}
                >
                  {modelData['analysis-vl'] ? '修改' : '配置'}
                </Button>
              </Stack>
            </Card>
          </>
        )}
        {addOpen && (
          <Suspense fallback={null}>
            <ModelModal
              open={addOpen}
              model_type={addType}
              data={
                addType === 'chat'
                  ? modelModalChatData
                  : addType === 'embedding'
                    ? modelModalEmbeddingData
                    : addType === 'rerank'
                      ? modelModalRerankData
                      : addType === 'analysis'
                        ? modelModalAnalysisData
                        : addType === 'analysis-vl'
                          ? modelModalAnalysisVLData
                          : null
              }
              onClose={() => {
                setAddOpen(false);
              }}
              refresh={async () => {
                setAddOpen(false);
                await getModelList();
              }}
              modelService={modelService}
              language='zh-CN'
              messageComponent={message}
              is_close_model_remark={true}
              addingModelTutorialURL=''
            />
          </Suspense>
        )}
      </Stack>
    );
  },
);

export default ModelConfig;
