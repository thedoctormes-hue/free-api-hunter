import { useQuery } from '@tanstack/react-query';
import { fetchTTSProviders, fetchTTSStats, TTSProvider, TTSStats } from '../api/tts';

export function useTTSProviders() {
  return useQuery<TTSProvider[]>({
    queryKey: ['tts-providers'],
    queryFn: fetchTTSProviders,
    staleTime: 5 * 60 * 1000, // 5 минут
  });
}

export function useTTSStats() {
  return useQuery<TTSStats>({
    queryKey: ['tts-stats'],
    queryFn: fetchTTSStats,
    staleTime: 5 * 60 * 1000,
  });
}
