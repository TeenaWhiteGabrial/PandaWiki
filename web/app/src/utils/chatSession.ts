export type ChatSession = {
  conversation_id: string;
  nonce: string;
  forked?: boolean;
};

type SessionStorage = Pick<Storage, 'getItem' | 'setItem'>;
export type HistoryCredential = { id: string; nonce: string };

export class ChatSessionMemory {
  private session: ChatSession = { conversation_id: '', nonce: '' };
  private needsResume = false;
  readonly generation = { current: 0 };
  private scope: string;
  private storage: () => SessionStorage;
  private onChange: (id: string) => void;
  private persistentStorage: () => SessionStorage;

  constructor(
    scope: string,
    storage: () => SessionStorage,
    onChange: (id: string) => void,
    persistentStorage: () => SessionStorage = storage,
  ) {
    this.scope = scope;
    this.storage = storage;
    this.onChange = onChange;
    this.persistentStorage = persistentStorage;
  }

  historyCredentials(): HistoryCredential[] {
    try {
      const entries: unknown = JSON.parse(
        this.persistentStorage().getItem(
          `panda_wiki_chat_history:${this.scope}`,
        ) || '[]',
      );
      if (!Array.isArray(entries)) return [];
      return entries
        .filter(
          (item): item is HistoryCredential =>
            !!item &&
            typeof item.id === 'string' &&
            item.id.length > 0 &&
            item.id.length <= 128 &&
            typeof item.nonce === 'string' &&
            item.nonce.length > 0 &&
            item.nonce.length <= 128,
        )
        .slice(0, 100);
    } catch {
      return [];
    }
  }

  private key(id: string) {
    return `panda_wiki_chat_nonce:${this.scope}:${id}`;
  }

  private save() {
    if (this.session.conversation_id && this.session.nonce) {
      try {
        this.storage().setItem(
          this.key(this.session.conversation_id),
          this.session.nonce,
        );
      } catch {}
      try {
        const entries = this.historyCredentials().filter(
          item => item.id !== this.session.conversation_id,
        );
        entries.unshift({
          id: this.session.conversation_id,
          nonce: this.session.nonce,
        });
        this.persistentStorage().setItem(
          `panda_wiki_chat_history:${this.scope}`,
          JSON.stringify(entries.slice(0, 100)),
        );
      } catch {}
    }
  }

  restore(id: string) {
    this.generation.current++;
    let nonce = '';
    try {
      nonce = this.storage().getItem(this.key(id)) || '';
    } catch {}
    if (!nonce)
      nonce =
        this.historyCredentials().find(item => item.id === id)?.nonce || '';
    this.session = { conversation_id: id, nonce };
    this.needsResume = true;
    if (nonce) this.save();
    this.onChange(id);
  }

  reset() {
    this.generation.current++;
    this.session = { conversation_id: '', nonce: '' };
    this.needsResume = false;
    this.onChange('');
  }

  acceptEvent(type: string, content: string) {
    if (type === 'conversation_id') {
      if (this.session.conversation_id !== content) this.session.nonce = '';
      this.session.conversation_id = content;
      this.onChange(content);
    } else if (type === 'nonce') {
      this.session.nonce = content;
    }
    this.save();
  }

  async resolve(
    resume: (id: string, nonce: string) => Promise<ChatSession>,
  ): Promise<ChatSession | null> {
    const version = this.generation.current;
    if (this.needsResume && this.session.conversation_id) {
      const session = await resume(
        this.session.conversation_id,
        this.session.nonce,
      );
      if (version !== this.generation.current) return null;
      if (!session?.conversation_id || !session.nonce)
        throw new Error('恢复会话失败，请重新打开历史对话');
      this.session = session;
      this.needsResume = false;
      this.onChange(session.conversation_id);
      this.save();
    }
    this.save();
    return { ...this.session };
  }
}
