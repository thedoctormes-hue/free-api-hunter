// TTS API клиент — запросы к /api/v1/tts/*

export interface TTSProvider {
  name: string;
  url: string;
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

const BASE = '/api/v1/tts';

export async function fetchTTSProviders(): Promise<TTSProvider[]> {
  const res = await fetch(`${BASE}/providers`);
  if (!res.ok) throw new Error(`TTS providers fetch failed: ${res.status}`);
  const data = await res.json();
  return data.data || [];
}

export async function fetchTTSProviderByID(id: string): Promise<TTSProvider> {
  const res = await fetch(`${BASE}/providers/${encodeURIComponent(id)}`);
  if (!res.ok) throw new Error(`TTS provider fetch failed: ${res.status}`);
  const data = await res.json();
  return data.data;
}

export async function fetchTTSStats(): Promise<TTSStats> {
  const res = await fetch(`${BASE}/stats`);
  if (!res.ok) throw new Error(`TTS stats fetch failed: ${res.status}`);
  const data = await res.json();
  return data.data;
}
