import httpRequest, { ContentType } from './httpClient';
import type { DomainResponse } from './types';

export const cancelPendingLearning = (kbId: string) =>
  httpRequest<
    DomainResponse & {
      data?: { count: number; protected_count: number; legacy_cutoff: number };
    }
  >({
    path: '/api/v1/node/learning/cancel-pending',
    method: 'POST',
    body: { kb_id: kbId },
    secure: true,
    type: ContentType.Json,
    format: 'json',
  });
