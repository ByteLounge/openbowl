import { useState, useEffect } from 'react';
import { 
  Bot, 
  Layers, 
  Settings, 
  FolderGit2, 
  BookMarked, 
  ListTodo, 
  Cpu, 
  Database,
  Send,
  Zap,
  Globe,
  Plus
} from 'lucide-react';
import { approximateTokens, formatCost } from '@openbowl/shared';
import { Provider, Message, Task, Memory } from '@openbowl/types';

// Mock Initial Data
const mockProviders: Provider[] = [
  { id: 'p-1', workspaceId: 'w-1', name: 'Anthropic Claude', providerType: 'anthropic', isActive: true, createdAt: '' },
  { id: 'p-2', workspaceId: 'w-1', name: 'Google Gemini', providerType: 'gemini', isActive: true, createdAt: '' },
  { id: 'p-3', workspaceId: 'w-1', name: 'OpenAI GPT-4', providerType: 'openai', isActive: true, createdAt: '' },
  { id: 'p-4', workspaceId: 'w-1', name: 'DeepSeek V3', providerType: 'deepseek', isActive: true, createdAt: '' }
];

const mockTasks: Task[] = [
  { id: 't-1', projectId: 'proj-1', title: 'Initialize OpenBowl Scaffold', status: 'completed', createdAt: '', updatedAt: '' },
  { id: 't-2', projectId: 'proj-1', title: 'Implement Universal Provider SDK', status: 'in_progress', createdAt: '', updatedAt: '' },
  { id: 't-3', projectId: 'proj-1', title: 'Build Context Aggregator Pipeline', status: 'todo', createdAt: '', updatedAt: '' }
];

const mockMemories: Memory[] = [
  { id: 'm-1', workspaceId: 'w-1', category: 'decision', content: 'Use CGO-free modernc.org SQLite driver for cross-platform portability.', isActive: true, createdAt: '', updatedAt: '' },
  { id: 'm-2', workspaceId: 'w-1', category: 'preference', content: 'Prefer functional style React components with modern CSS styling.', isActive: true, createdAt: '', updatedAt: '' }
];

export default function App() {
  const [activeProviderIndex, setActiveProviderIndex] = useState(0);
  const [inputText, setInputText] = useState('');
  const [messages, setMessages] = useState<Message[]>([
    {
      id: 'msg-1',
      conversationId: 'c-1',
      role: 'system',
      content: 'System: Injected OpenBowl workspace context and active decisions.',
      tokensPrompt: 0,
      tokensCompletion: 0,
      cost: 0,
      status: 'sent',
      createdAt: ''
    },
    {
      id: 'msg-2',
      conversationId: 'c-1',
      role: 'user',
      content: 'Explain how OpenBowl maintains context when switching models.',
      tokensPrompt: 0,
      tokensCompletion: 0,
      cost: 0,
      status: 'sent',
      createdAt: ''
    },
    {
      id: 'msg-3',
      conversationId: 'c-1',
      role: 'assistant',
      content: 'OpenBowl decouples memory and task tracking from LLM clients. When switching providers, the Context Engine formats the current file links, task board, and workspace decisions, presenting a compact, unified prompt context package to the newly selected provider.',
      tokensPrompt: 0,
      tokensCompletion: 0,
      cost: 0,
      status: 'sent',
      createdAt: ''
    }
  ]);
  const [websocketStatus, setWebsocketStatus] = useState<'disconnected' | 'connecting' | 'connected'>('disconnected');
  const [costAccumulated, setCostAccumulated] = useState(0.00342);

  const activeProvider = mockProviders[activeProviderIndex];

  // Cycle providers simulation
  const handleSwitchProvider = () => {
    setActiveProviderIndex((prev) => (prev + 1) % mockProviders.length);
  };

  // Simulating connection to Go backend sidecar WebSocket
  useEffect(() => {
    setWebsocketStatus('connecting');
    const ws = new WebSocket('ws://localhost:3010/ws/chat');

    ws.onopen = () => {
      setWebsocketStatus('connected');
    };

    ws.onerror = () => {
      setWebsocketStatus('disconnected');
    };

    ws.onclose = () => {
      setWebsocketStatus('disconnected');
    };

    return () => {
      ws.close();
    };
  }, []);

  const handleSendMessage = () => {
    if (!inputText.trim()) return;

    const userMsg: Message = {
      id: `msg-u-${Date.now()}`,
      conversationId: 'c-1',
      role: 'user',
      content: inputText,
      tokensPrompt: approximateTokens(inputText),
      tokensCompletion: 0,
      cost: 0,
      status: 'sent',
      createdAt: new Date().toISOString()
    };

    setMessages((prev) => [...prev, userMsg]);
    setInputText('');

    // Dynamic cost simulation
    setCostAccumulated((prev) => prev + 0.00018);

    // Simulated Response
    setTimeout(() => {
      const responseText = `Received by OpenBowl orchestration layer using current provider: ${activeProvider.name}. The context remains secure and active.`;
      const replyMsg: Message = {
        id: `msg-r-${Date.now()}`,
        conversationId: 'c-1',
        role: 'assistant',
        content: responseText,
        tokensPrompt: 0,
        tokensCompletion: approximateTokens(responseText),
        cost: 0.00024,
        status: 'sent',
        createdAt: new Date().toISOString()
      };
      setMessages((prev) => [...prev, replyMsg]);
    }, 800);
  };

  return (
    <div className="app-container">
      {/* SIDEBAR */}
      <aside className="sidebar">
        <div style={{ padding: '24px', display: 'flex', alignItems: 'center', gap: '12px', borderBottom: '1px solid var(--border-subtle)' }}>
          <div style={{ backgroundColor: 'var(--accent-purple)', padding: '6px', borderRadius: '8px' }}>
            <Layers size={18} color="white" />
          </div>
          <div>
            <h2 style={{ fontSize: '15px', fontWeight: 600 }}>OpenBowl Workspace</h2>
            <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Universal Memory Layer</span>
          </div>
        </div>

        {/* Project Lists */}
        <div style={{ flex: 1, padding: '16px', overflowY: 'auto' }}>
          <div style={{ marginBottom: '24px' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
              <span style={{ fontSize: '11px', fontWeight: 600, textTransform: 'uppercase', tracking: '0.05em', color: 'var(--text-muted)' }}>Projects</span>
              <Plus size={14} style={{ cursor: 'pointer', color: 'var(--text-muted)' }} />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px', borderRadius: '6px', backgroundColor: 'var(--bg-tertiary)', fontSize: '13px', cursor: 'pointer' }}>
                <FolderGit2 size={15} color="var(--accent-purple)" />
                <span>OpenBowl Core</span>
              </div>
            </div>
          </div>

          {/* Tasks Panel */}
          <div style={{ marginBottom: '24px' }}>
            <span style={{ fontSize: '11px', fontWeight: 600, textTransform: 'uppercase', color: 'var(--text-muted)', display: 'block', marginBottom: '8px' }}>Tasks</span>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              {mockTasks.map((t) => (
                <div key={t.id} style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '12px' }}>
                  <input type="checkbox" checked={t.status === 'completed'} readOnly style={{ accentColor: 'var(--accent-purple)' }} />
                  <span style={{ color: t.status === 'completed' ? 'var(--text-muted)' : 'var(--text-secondary)', textDecoration: t.status === 'completed' ? 'line-through' : 'none' }}>
                    {t.title}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Extracted Memories */}
          <div>
            <span style={{ fontSize: '11px', fontWeight: 600, textTransform: 'uppercase', color: 'var(--text-muted)', display: 'block', marginBottom: '8px' }}>Extracted Decisions</span>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {mockMemories.map((m) => (
                <div key={m.id} style={{ padding: '8px', borderRadius: '6px', borderLeft: '2px solid var(--accent-purple)', backgroundColor: 'rgba(255,255,255,0.02)', fontSize: '11px', color: 'var(--text-secondary)' }}>
                  {m.content}
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div style={{ padding: '16px', borderTop: '1px solid var(--border-subtle)', display: 'flex', flexDirection: 'column', gap: '8px' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', fontSize: '11px', color: 'var(--text-muted)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <Globe size={12} color={websocketStatus === 'connected' ? 'var(--status-success)' : 'var(--status-error)'} />
              <span>Go Sidecar: {websocketStatus}</span>
            </div>
            <span>v0.1.0</span>
          </div>
        </div>
      </aside>

      {/* CANVAS */}
      <main className="workspace-canvas">
        {/* Header Bar */}
        <header style={{ height: '64px', borderBottom: '1px solid var(--border-subtle)', display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0 24px', backgroundColor: 'var(--bg-secondary)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <span style={{ fontSize: '14px', fontWeight: 500 }}>Active Model:</span>
            <button 
              onClick={handleSwitchProvider}
              style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 12px', border: '1px solid var(--border-subtle)', borderRadius: '6px', backgroundColor: 'var(--bg-tertiary)', color: 'var(--text-primary)', cursor: 'pointer', fontSize: '13px', transition: 'all 0.2s' }}
            >
              <Cpu size={14} color="var(--accent-purple)" />
              <span>{activeProvider.name}</span>
            </button>
            <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>(Click button to switch provider)</span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '16px', fontSize: '13px', color: 'var(--text-secondary)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <Zap size={14} color="var(--status-warning)" />
              <span>Cost: {formatCost(costAccumulated)}</span>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <Database size={14} color="var(--status-success)" />
              <span>Decisions: {mockMemories.length}</span>
            </div>
          </div>
        </header>

        {/* Conversation List */}
        <div style={{ flex: 1, padding: '24px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {messages.map((msg) => (
            <div 
              key={msg.id} 
              style={{ 
                alignSelf: msg.role === 'user' ? 'flex-end' : 'flex-start',
                maxWidth: '70%',
                padding: '12px 16px',
                borderRadius: '12px',
                backgroundColor: msg.role === 'user' ? 'var(--accent-purple)' : 'var(--bg-secondary)',
                border: msg.role === 'user' ? 'none' : '1px solid var(--border-subtle)',
                color: 'var(--text-primary)',
                fontSize: '14px',
                lineHeight: 1.5,
                boxShadow: msg.role === 'user' ? '0 4px 12px rgba(139, 92, 246, 0.2)' : 'none'
              }}
            >
              <div>{msg.content}</div>
              {msg.tokensPrompt > 0 && (
                <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '6px', fontSize: '9px', color: msg.role === 'user' ? 'rgba(255,255,255,0.7)' : 'var(--text-muted)' }}>
                  Tokens: {msg.tokensPrompt}
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Input Dock */}
        <div style={{ padding: '24px', borderTop: '1px solid var(--border-subtle)', backgroundColor: 'var(--bg-secondary)' }}>
          <div style={{ display: 'flex', gap: '12px', position: 'relative' }}>
            <input 
              type="text"
              placeholder="Ask anything... Use /goal to run autonomous agent tasks"
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSendMessage()}
              style={{ flex: 1, backgroundColor: 'var(--bg-tertiary)', border: '1px solid var(--border-subtle)', borderRadius: '8px', padding: '14px 16px', color: 'var(--text-primary)', fontSize: '14px', outline: 'none', transition: 'border-color 0.2s' }}
            />
            <button 
              onClick={handleSendMessage}
              style={{ padding: '0 16px', borderRadius: '8px', border: 'none', backgroundColor: 'var(--accent-purple)', color: 'white', cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'center' }}
            >
              <Send size={16} />
            </button>
          </div>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginTop: '8px', fontSize: '11px', color: 'var(--text-muted)' }}>
            <span>Active context: 1 file reference, 2 active memories</span>
            <span>Press Enter to send</span>
          </div>
        </div>
      </main>
    </div>
  );
}
