import { TTSStats } from '../../api/tts';

interface Props {
  stats: TTSStats;
}

export function TTSStatsCards({ stats }: Props) {
  const cards = [
    { label: 'Провайдеров', value: stats.providers_total, icon: '🎙' },
    { label: 'Активных', value: stats.active_count, icon: '✅' },
    { label: 'С Free Tier', value: stats.free_tier_count, icon: '🆓' },
    { label: 'Голосов', value: stats.total_voices, icon: '🗣' },
  ];

  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
      {cards.map((card) => (
        <div
          key={card.label}
          className="bg-[#161b22] border border-[#30363d] rounded-lg p-4 text-center"
        >
          <div className="text-2xl mb-1">{card.icon}</div>
          <div className="text-2xl font-bold text-[#c9d1d9]">{card.value}</div>
          <div className="text-xs text-[#8b949e] mt-1">{card.label}</div>
        </div>
      ))}
    </div>
  );
}
