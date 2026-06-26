// TTS API клиент — запросы к /api/v1/tts/*
import { fetchJSON } from './client';

export interface TTSProvider {
  name: string;
  url?: string;
  api_key_url: string;
  credit_card: boolean;
  status: string;
  models: string[];
  limits: string;
  free_tier?: {
    char_limit: number;
    voice_clones: number;
    reset_period: string;
  };
  features: string[];
  languages: string[];
  source: string;
  priority: number;
  discovered_at: string;
  last_verified?: string;
  notes: string;
}

export interface TTSVerifyResult {
  is_active: boolean;
  status_code: number;
  error?: string;
  models: string[];
  voices: string[];
  plan: string;
  char_limit: number;
  checked_at: string;
}

export interface TTSScore {
  provider_name: string;
  overall_score: number;
  free_tier_score: number;
  feature_score: number;
  language_score: number;
  latency_score: number;
  has_free_tier: boolean;
  char_limit: number;
  scored_at: string;
}

export interface TTSStats {
  providers_total: number;
  active_count: number;
  free_tier_count: number;
  total_voices: number;
  updated_at: string;
}

export async function fetchTTSProviders(): Promise<TTSProvider[]> {
  return fetchJSON<TTSProvider[]>('/api/v1/tts/providers');
}

export async function fetchTTSProviderByID(id: string): Promise<TTSProvider> {
  return fetchJSON<TTSProvider>(`/api/v1/tts/providers/${encodeURIComponent(id)}`);
}

export async function fetchTTSStats(): Promise<TTSStats> {
  return fetchJSON<TTSStats>('/api/v1/tts/stats');
}
