import { useTTSProviders, useTTSStats } from '../hooks/use-tts';
import { TTSProviderCard } from '../components/tts/tts-card';
import { TTSStatsCards } from '../components/tts/tts-stats';

export default function TTSPage() {
  const { data: providers, isLoading: providersLoading, error: providersError } = useTTSProviders();
  const { data: stats, isLoading: statsLoading } = useTTSStats();

  return (
    <div className="min-h-screen bg-[#0d1117] text-[#c9d1d9]">
      <div className="max-w-7xl mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-[#58a6ff]">🎙 TTS/STT Providers</h1>
          <p className="text-[#8b949e] mt-2">
            Text-to-Speech и Speech-to-Text API — каталог бесплатных и условно-бесплатных сервисов озвучки
          </p>
        </div>

          {/* Stats Cards */}
          {statsLoading ? (
            <LoadingSpinner />
          ) : stats ? (
            <TTSStatsCards stats={stats} />
          ) : null}

          {/* Providers List */}
          <section className="mt-8">
            <h2 className="text-xl font-semibold text-[#79c0ff] mb-4">
              Провайдеры
            </h2>

            {providersLoading && (
              <div className="flex items-center justify-center py-12">
                <div className="animate-spin w-8 h-8 border-2 border-[#58a6ff] border-t-transparent rounded-full" />
                <span className="ml-3 text-[#8b949e]">Загрузка провайдеров...</span>
              </div>
            )}

            {providersError && (
              <div className="bg-[#161b22] border border-[#f85149] rounded-lg p-4 text-[#f85149]">
                Ошибка загрузки: {providersError.message}
              </div>
            )}

            {providers && providers.length === 0 && (
              <div className="bg-[#161b22] border border-[#30363d] rounded-lg p-8 text-center text-[#8b949e]">
                <p className="text-lg">Нет доступных TTS-провайдеров</p>
                <p className="text-sm mt-2">
                  Запустите <code className="bg-[#0d1117] px-2 py-1 rounded">hunter --verify</code> для сканирования
                </p>
              </div>
            )}

            {providers && providers.length > 0 && (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {providers.map((provider) => (
                  <TTSProviderCard key={provider.name} provider={provider} />
                ))}
              </div>
            )}
          </section>

          {/* Footer note */}
          <footer className="mt-12 pt-6 border-t border-[#21262d] text-center text-[#484f58] text-sm">
            <p>
              Данные обновляются при запуске с флагом <code className="bg-[#161b22] px-1 rounded">--verify</code>
            </p>
            {stats?.updated_at && (
              <p className="mt-1">
                Последнее обновление: {new Date(stats.updated_at).toLocaleString('ru-RU')}
              </p>
            )}
          </footer>
        </div>
      </div>
      </div>
    </div>
  );
}
