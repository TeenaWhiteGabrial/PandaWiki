import { ModelProvider } from '@/constant/enums';
import { postApiV1ModelProviderSupported } from '@/request/Model';
import {
  AUTO_MODE_PROVIDER_KEYS,
  normalizeModelProviderForApi,
  normalizeModelProviderForDisplay,
} from '@/utils/modelProvider';
import { Select } from 'antd';
import {
  Box,
  Button,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { Visibility, VisibilityOff } from '@mui/icons-material';
import InfoOutlineSharpIcon from '@mui/icons-material/InfoOutlineSharp';
import {
  forwardRef,
  useEffect,
  useImperativeHandle,
  useMemo,
  useRef,
  useState,
} from 'react';

type ModelTypeKey =
  | 'chat'
  | 'embedding'
  | 'rerank'
  | 'analysis'
  | 'analysis-vl';

export interface AutoModelConfigFormData {
  provider: string;
  base_url: string;
  api_key: string;
  models: {
    chat: string;
    embedding: string;
    rerank: string;
    analysis: string;
    analysis_vl: string;
  };
}

export interface AutoModelConfigRef {
  getFormData: () => AutoModelConfigFormData;
}

interface AutoModelConfigProps {
  showTip?: boolean;
  initialValue?: AutoModelConfigFormData | null;
  onDataChange?: () => void;
}

const AUTO_MODEL_TYPE_FIELDS: {
  key: ModelTypeKey;
  label: string;
  responseKey: keyof AutoModelConfigFormData['models'];
}[] = [
  { key: 'chat', label: '智能对话', responseKey: 'chat' },
  { key: 'embedding', label: '向量模型', responseKey: 'embedding' },
  { key: 'rerank', label: '重排序', responseKey: 'rerank' },
  { key: 'analysis', label: '文档分析', responseKey: 'analysis' },
  { key: 'analysis-vl', label: '视觉分析', responseKey: 'analysis_vl' },
];

const buildProviderOptionList = () => {
  return AUTO_MODE_PROVIDER_KEYS.map(key => {
    const value = ModelProvider[key];
    return {
      value: key,
      label: value.cn || value.label || key,
      defaultBaseUrl: value.defaultBaseUrl || '',
    };
  });
};

const createDefaultValue = (): AutoModelConfigFormData => ({
  provider: 'BaiZhiCloud',
  base_url: ModelProvider.BaiZhiCloud.defaultBaseUrl || '',
  api_key: '',
  models: {
    chat: '',
    embedding: '',
    rerank: '',
    analysis: '',
    analysis_vl: '',
  },
});

const normalizeFormData = (
  value: AutoModelConfigFormData | null,
): AutoModelConfigFormData => {
  const next = value || createDefaultValue();
  const provider = normalizeModelProviderForDisplay(next.provider);
  return {
    ...next,
    provider,
    base_url: next.base_url || ModelProvider[provider]?.defaultBaseUrl || '',
    models: {
      chat: next.models?.chat || '',
      embedding: next.models?.embedding || '',
      rerank: next.models?.rerank || '',
      analysis: next.models?.analysis || '',
      analysis_vl: next.models?.analysis_vl || '',
    },
  };
};

const AutoModelConfig = forwardRef<AutoModelConfigRef, AutoModelConfigProps>(
  (props, ref) => {
    const { showTip = false, initialValue = null, onDataChange } = props;
    const providerOptions = useMemo(buildProviderOptionList, []);
    const [formData, setFormData] = useState<AutoModelConfigFormData>(
      normalizeFormData(initialValue),
    );
    const [modelOptions, setModelOptions] = useState<
      Record<ModelTypeKey, string[]>
    >({
      chat: [],
      embedding: [],
      rerank: [],
      analysis: [],
      'analysis-vl': [],
    });
    const [loadingType, setLoadingType] = useState<ModelTypeKey | null>(null);
    const [showApiKey, setShowApiKey] = useState(false);
    const fetchTokenRef = useRef(0);
    const provider = formData.provider;
    const baseURL = formData.base_url;
    const apiKey = formData.api_key;

    useEffect(() => {
      setFormData(normalizeFormData(initialValue));
    }, [initialValue]);

    useImperativeHandle(ref, () => ({
      getFormData: () => formData,
    }));

    const updateField = <K extends keyof AutoModelConfigFormData>(
      key: K,
      value: AutoModelConfigFormData[K],
    ) => {
      setFormData(current => ({ ...current, [key]: value }));
      onDataChange?.();
    };

    const updateModelField = (
      key: keyof AutoModelConfigFormData['models'],
      value: string,
    ) => {
      setFormData(current => ({
        ...current,
        models: {
          ...current.models,
          [key]: value,
        },
      }));
      onDataChange?.();
    };

    const fetchModelsByType = async (
      provider: string,
      baseURL: string,
      apiKey: string,
      modelType: ModelTypeKey,
      token: number,
    ) => {
      setLoadingType(modelType);
      try {
        const result = await postApiV1ModelProviderSupported({
          provider: normalizeModelProviderForApi(provider),
          base_url: baseURL,
          api_key: apiKey,
          type: modelType,
        });
        if (fetchTokenRef.current !== token) {
          return;
        }
        const nextModels = (result.models || [])
          .map(item => item.model || '')
          .filter(Boolean);
        const responseKey = AUTO_MODEL_TYPE_FIELDS.find(
          field => field.key === modelType,
        )?.responseKey;
        setModelOptions(current => ({
          ...current,
          [modelType]: nextModels,
        }));
        if (responseKey && nextModels.length > 0) {
          setFormData(current => {
            if (current.models[responseKey]) {
              return current;
            }
            return {
              ...current,
              models: {
                ...current.models,
                [responseKey]: nextModels[0],
              },
            };
          });
        }
      } finally {
        if (fetchTokenRef.current === token) {
          setLoadingType(null);
        }
      }
    };

    useEffect(() => {
      if (!provider || !baseURL) {
        setModelOptions({
          chat: [],
          embedding: [],
          rerank: [],
          analysis: [],
          'analysis-vl': [],
        });
        return;
      }
      const token = Date.now();
      fetchTokenRef.current = token;
      void Promise.all(
        AUTO_MODEL_TYPE_FIELDS.map(item =>
          fetchModelsByType(provider, baseURL, apiKey, item.key, token),
        ),
      );
    }, [provider, baseURL, apiKey]);

    return (
      <Stack
        sx={{
          flex: 1,
          p: 2,
          pl: 2,
          pr: 0,
          pt: 0,
          overflow: 'hidden',
          overflowY: 'auto',
        }}
      >
        {showTip && (
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              gap: 1,
              p: 1.5,
              mb: 2,
              bgcolor: 'rgba(25, 118, 210, 0.08)',
              borderRadius: '8px',
              border: '1px solid rgba(25, 118, 210, 0.2)',
            }}
          >
            <InfoOutlineSharpIcon
              sx={{
                fontSize: 16,
                color: 'primary.main',
                flexShrink: 0,
                mt: 0.2,
              }}
            />
            <Box
              sx={{ fontSize: 12, lineHeight: 1.6, color: 'text.secondary' }}
            >
              自动模式下统一配置供应商、Base URL 和 API
              Key，再为智能对话、向量模型、重排序、文档分析等过程分别选择模型。
            </Box>
          </Box>
        )}

        <Stack gap={2}>
          <Typography fontSize={14} fontWeight={700}>
            自动配置
          </Typography>

          <Stack direction='row' gap={2}>
            <Box sx={{ width: 220 }}>
              <Typography fontSize={12} color='text.secondary' mb={0.5}>
                供应商
              </Typography>
              <Select
                style={{ width: '100%' }}
                size='middle'
                value={formData.provider}
                onChange={nextProvider => {
                  updateField('provider', nextProvider);
                  updateField(
                    'base_url',
                    ModelProvider[
                      normalizeModelProviderForDisplay(nextProvider)
                    ]?.defaultBaseUrl || '',
                  );
                }}
                options={providerOptions.map(option => ({
                  value: option.value,
                  label: option.label,
                }))}
              />
            </Box>
            <Box sx={{ flex: 1 }}>
              <Typography fontSize={12} color='text.secondary' mb={0.5}>
                Base URL
              </Typography>
              <TextField
                fullWidth
                size='small'
                value={formData.base_url}
                onChange={event => updateField('base_url', event.target.value)}
              />
            </Box>
          </Stack>

          <Box>
            <Typography fontSize={12} color='text.secondary' mb={0.5}>
              API Key
            </Typography>
            <TextField
              fullWidth
              size='small'
              value={formData.api_key}
              type={showApiKey ? 'text' : 'password'}
              onChange={event => updateField('api_key', event.target.value)}
              InputProps={{
                endAdornment: (
                  <InputAdornment position='end'>
                    <IconButton
                      size='small'
                      onClick={() => setShowApiKey(value => !value)}
                      edge='end'
                      aria-label={showApiKey ? '隐藏 API Key' : '显示 API Key'}
                    >
                      {showApiKey ? <VisibilityOff /> : <Visibility />}
                    </IconButton>
                  </InputAdornment>
                ),
              }}
            />
          </Box>

          <Stack gap={2}>
            {AUTO_MODEL_TYPE_FIELDS.map(item => (
              <Stack key={item.key} direction='row' alignItems='center' gap={2}>
                <Box sx={{ width: 120, fontSize: 12, color: 'text.secondary' }}>
                  {item.label}
                </Box>
                <Select
                  style={{ width: '100%' }}
                  size='middle'
                  value={formData.models[item.responseKey] || ''}
                  onChange={value => updateModelField(item.responseKey, value)}
                  placeholder={`请选择${item.label}模型`}
                  options={modelOptions[item.key].map(model => ({
                    value: model,
                    label: model,
                  }))}
                />
                {loadingType === item.key && (
                  <Button size='small' disabled>
                    获取中
                  </Button>
                )}
              </Stack>
            ))}
          </Stack>
        </Stack>
      </Stack>
    );
  },
);

AutoModelConfig.displayName = 'AutoModelConfig';

export default AutoModelConfig;
