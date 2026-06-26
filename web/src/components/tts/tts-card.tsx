import { useState } from 'react';
import { TTSProvider } from '../../api/tts';

interface Props {
  provider: TTSProvider;
}

export function TTSProviderCard({ provider }: Props) {
  const [expanded, setExpanded] = useState(false);

  const statusColor = provider.status === 'active' || provider.status === 'verified'
    ? 'text-[#3fb950]'
    : provider.status === 'expired'
      ? 'text-[#f85149]'
      : 'text-[#d29922]';

  const statusIcon = provider.status === 'active' || provider.status === 'verified'
    ? '✅'
    : provider.status === 'expired'
      ? '❌'
      : '⚠️';

  return (
    <div className="bg-[#161b22] border border-[#30363d] rounded-lg p-4 hover:border-[#58a6ff] transition-colors">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h3 className="text-lg font-semibold text-[#c9d1d9]">{provider.name}</h3>
          <span className={`text-sm ${statusColor}`}>
            {statusIcon} {provider.status}
          </span>
        </div>
        {provider.free_tier && (
          <span className="bg-[#0d1117] text-[#58a6ff] text-xs px-2 py-1 rounded-full border border-[#30363d]">
            Free: {provider.free_tier.char_limit.toLocaleString()} chars
          </span>
        )}
      </div>

      {/* Models */}
      {provider.models.length > 0 && (
        <div className="mt-3">
          <span className="text-xs text-[#8b949e]">Модели:</span>
          <div className="flex flex-wrap gap-1 mt-1">
            {provider.models.slice(0, 4).map((m) => (
              <span key={m} className="bg-[#0d1117] text-[#c9d1d9] text-xs px-2 py-0.5 rounded border border-[#30363d]">
                {m}
              </span>
            ))}
            {provider.models.length > 4 && (
              <span className="text-xs text-[#484f58]">+{provider.models.length - 4}</span>
            )}
          </div>
        </div>
      )}

      {/* Features */}
      {provider.features.length > 0 && (
        <div className="mt-3">
          <span className="text-xs text-[#8b949e]">Фичи:</span>
          <div className="flex flex-wrap gap-1 mt-1">
            {provider.features.map((f) => (
              <span key={f} className="bg-[#0d4429] text-[#3fb950] text-xs px-2 py-0.5 rounded">
                {f}
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Expandable details */}
      <button
        onClick={() => setExpanded(!expanded)}
        className="mt-3 text-xs text-[#58a6ff] hover:underline"
      >
        {expanded ? 'Скрыть' : 'Подробнее'}
      </button>

      {expanded && (
        <div className="mt-3 pt-3 border-t border-[#21262d] text-sm space-y-2">
          {provider.limits && (
            <div>
              <span className="text-[#8b949e]">Лимиты:</span>{' '}
              <span className="text-[#c9d1d9]">{provider.limits}</span>
            </div>
          )}
          {provider.languages.length > 0 && (
            <div>
              <span className="text-[#8b949e]">Языки:</span>{' '}
              <span className="text-[#c9d1d9]">{provider.languages.join(', ')}</span>
            </div>
          )}
          {provider.free_tier && (
            <div>
              <span className="text-[#8b949e]">Free tier:</span>{' '}
              <span className="text-[#c9d1d9]">
                {provider.free_tier.char_limit.toLocaleString()} chars/{provider.free_tier.reset_period},{' '}
                {provider.free_tier.voice_clones} voice clones
              </span>
            </div>
          )}
          {provider.notes && (
            <div>
              <span className="text-[#8b949e]">Заметки:</span>{' '}
              <span className="text-[#c9d1d9]">{provider.notes}</span>
            </div>
          )}
          {provider.url && (
            <div>
              <span className="text-[#8b949e]">URL:</span>{' '}
              <a href={provider.url} target="_blank" rel="noopener noreferrer" className="text-[#58a6ff] hover:underline">
                {provider.url}
              </a>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
