import Card from '@/components/Card';
import Member from '@/components/System/component/Member';
import MCPAccess from '@/components/System/component/MCPAccess';
import ModelConfig from '@/components/System/component/ModelConfig';
import EnterpriseAccess from '@/components/System/component/EnterpriseAccess';
import CardAssistantTemplates from './component/CardAssistantTemplates';
import CardRuntimeLogs from './component/CardRuntimeLogs';
import { getApiV1KnowledgeBaseDetail } from '@/request/KnowledgeBase';
import { getApiV1ModelList } from '@/request/Model';
import { GithubComChaitinPandaWikiDomainModelListItem } from '@/request/types';
import { ConstsUserRole } from '@/request/types';
import { DomainKnowledgeBaseDetail } from '@/request/types';
import { useAppDispatch, useAppSelector } from '@/store';
import {
  setKbDetail,
  setModelList,
  setModelStatus,
} from '@/store/slices/config';
import { Box, Tab, Tabs } from '@mui/material';
import { useCallback, useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import CardFeedback from './component/CardFeedback';
import CardWeb from './component/CardWeb';

const SettingTabs: { label: string; id: string }[] = [
  { label: '门户网站', id: 'portal-website' },
  { label: '反馈设置', id: 'feedback' },
  { label: '模型配置', id: 'model-config' },
  { label: 'MCP 接入', id: 'mcp-access' },
  { label: '用户管理', id: 'user-management' },
  { label: '组织和权限', id: 'enterprise-access' },
  { label: '运行日志', id: 'runtime-logs' },
  { label: '智能助手设置', id: 'assistant-settings' },
];

const Setting = () => {
  const dispatch = useAppDispatch();
  const { kb_id, modelList, user } = useAppSelector(state => state.config);
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTab = searchParams.get('tab') ?? 'portal-website';
  const [kb, setKb] = useState<DomainKnowledgeBaseDetail | null>(null);
  const [chatModelData, setChatModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);
  const [embeddingModelData, setEmbeddingModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);
  const [rerankModelData, setRerankModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);
  const [analysisModelData, setAnalysisModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);
  const [analysisVLModelData, setAnalysisVLModelData] =
    useState<GithubComChaitinPandaWikiDomainModelListItem | null>(null);

  const tabs =
    user?.role === ConstsUserRole.UserRoleManager
      ? SettingTabs.filter(tab =>
          ['user-management', 'mcp-access'].includes(tab.id),
        )
      : user?.role === ConstsUserRole.UserRoleUser
        ? SettingTabs.filter(tab => tab.id === 'mcp-access')
        : SettingTabs;
  const legacyDestination =
    user?.role === ConstsUserRole.UserRoleManager
      ? 'user-management'
      : 'enterprise-access';
  const activeTab =
    tabs.find(tab => tab.id === requestedTab)?.id ??
    (requestedTab === 'permission-settings'
      ? tabs.find(tab => tab.id === legacyDestination)?.id
      : undefined) ??
    tabs[0].id;

  const getKb = () => {
    if (!kb_id) return;
    getApiV1KnowledgeBaseDetail({ id: kb_id }).then(res => {
      setKb(res);
      dispatch(setKbDetail(res));
    });
  };

  const getModelList = () => {
    getApiV1ModelList().then(res => {
      dispatch(
        setModelList(res as GithubComChaitinPandaWikiDomainModelListItem[]),
      );
    });
  };

  const handleModelList = useCallback(
    (list: GithubComChaitinPandaWikiDomainModelListItem[]) => {
      const chat = list.find(it => it.type === 'chat') || null;
      const embedding = list.find(it => it.type === 'embedding') || null;
      const rerank = list.find(it => it.type === 'rerank') || null;
      const analysis = list.find(it => it.type === 'analysis') || null;
      const analysisVL = list.find(it => it.type === 'analysis-vl') || null;
      setChatModelData(chat);
      setEmbeddingModelData(embedding);
      setRerankModelData(rerank);
      setAnalysisModelData(analysis);
      setAnalysisVLModelData(analysisVL);

      dispatch(setModelStatus(!!(chat && embedding && rerank)));
    },
    [dispatch],
  );

  const setActiveTab = useCallback(
    (tab: string, replace = false) => {
      setSearchParams(
        previous => {
          const next = new URLSearchParams(previous);
          next.set('tab', tab);
          return next;
        },
        { replace },
      );
    },
    [setSearchParams],
  );

  useEffect(() => {
    if (activeTab !== requestedTab) {
      setActiveTab(activeTab, true);
    }
  }, [activeTab, requestedTab, setActiveTab]);

  useEffect(() => {
    if (kb_id) getKb();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kb_id]);

  useEffect(() => {
    if (activeTab === 'model-config') {
      getModelList();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeTab]);

  useEffect(() => {
    if (modelList) {
      handleModelList(modelList);
    }
  }, [modelList, handleModelList]);

  if (
    user?.role !== ConstsUserRole.UserRoleManager &&
    user?.role !== ConstsUserRole.UserRoleUser &&
    !kb
  )
    return <></>;

  return (
    <Box
      sx={{
        position: 'relative',
      }}
    >
      <Card sx={{ mb: 2, display: 'flex', justifyContent: 'center' }}>
        <Tabs
          value={activeTab}
          onChange={(event, newValue) => setActiveTab(newValue as string)}
          aria-label='setting tabs'
        >
          {tabs.map(tab => (
            <Tab key={tab.id} label={tab.label} value={tab.id} />
          ))}
        </Tabs>
      </Card>
      <Card
        sx={{
          height: 'calc(100vh - 148px)',
          overflow: 'auto',
        }}
      >
        {activeTab === 'feedback' && kb && <CardFeedback kb={kb} />}
        {activeTab === 'portal-website' && kb && (
          <CardWeb kb={kb} refresh={getKb} />
        )}
        {activeTab === 'user-management' && <Member />}
        {activeTab === 'mcp-access' && <MCPAccess />}
        {activeTab === 'enterprise-access' && <EnterpriseAccess />}
        {activeTab === 'runtime-logs' &&
          user?.role === ConstsUserRole.UserRoleAdmin && <CardRuntimeLogs />}
        {activeTab === 'assistant-settings' &&
          user?.role === ConstsUserRole.UserRoleAdmin && (
            <CardAssistantTemplates />
          )}
        {activeTab === 'model-config' && (
          <ModelConfig
            onCloseModal={() => undefined}
            chatModelData={chatModelData}
            embeddingModelData={embeddingModelData}
            rerankModelData={rerankModelData}
            analysisModelData={analysisModelData}
            analysisVLModelData={analysisVLModelData}
            getModelList={getModelList}
            showSaveBtn={true}
          />
        )}
      </Card>
    </Box>
  );
};
export default Setting;
