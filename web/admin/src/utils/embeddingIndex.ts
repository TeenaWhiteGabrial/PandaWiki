export interface EmbeddingIndexConfig {
  provider: string;
  model: string;
  context_window: number;
  max_tokens: number;
}

type AutoEmbeddingConfig = {
  provider?: string;
  models?: { embedding?: string };
};

const providerName = (value = '') => {
  const name = value.trim();
  return name === 'Other' ? 'CompatiblePlatform' : name;
};

export function autoEmbeddingIndexConfig(
  config?: AutoEmbeddingConfig | null,
): EmbeddingIndexConfig {
  return {
    provider: providerName(config?.provider),
    model: config?.models?.embedding?.trim() || '',
    context_window: 0,
    max_tokens: 8192,
  };
}

export function embeddingIndexNeedsRebuild(
  baseline: EmbeddingIndexConfig | null | undefined,
  next: EmbeddingIndexConfig,
): boolean {
  if (!baseline) return true;
  return (
    providerName(baseline.provider) !== providerName(next.provider) ||
    baseline.model.trim() !== next.model.trim() ||
    baseline.context_window !== next.context_window ||
    (baseline.max_tokens || 8192) !== (next.max_tokens || 8192)
  );
}
