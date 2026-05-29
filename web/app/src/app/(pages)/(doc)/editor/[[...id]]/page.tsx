export default function EditorPage() {
  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
        color: '#1f2937',
        background: '#f8fafc',
        textAlign: 'center',
      }}
    >
      <div>
        <div
          style={{ fontSize: '24px', fontWeight: 700, marginBottom: '12px' }}
        >
          当前环境未启用在线投稿能力
        </div>
        <div style={{ fontSize: '14px', color: '#6b7280' }}>
          如需开放在线投稿，请在私有化改造完成后再单独接入对应流程。
        </div>
      </div>
    </div>
  );
}
