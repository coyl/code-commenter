"use client";

import { useCallback, useRef } from "react";

export const LIVE_SAMPLE_RATE = 24000;

export function usePCMPlayer() {
  const ctxRef = useRef<AudioContext | null>(null);
  const nextStartRef = useRef(0);
  const activeSourcesRef = useRef<Set<AudioBufferSourceNode>>(new Set());
  const unlockInFlightRef = useRef<Promise<void> | null>(null);

  const ensureContext = useCallback((): AudioContext => {
    if (ctxRef.current) return ctxRef.current;
    const Ctx =
      window.AudioContext ||
      (window as unknown as { webkitAudioContext: typeof AudioContext })
        .webkitAudioContext;
    const ctx = new Ctx();
    ctxRef.current = ctx;
    nextStartRef.current = ctx.currentTime;
    return ctx;
  }, []);

  const unlock = useCallback((): Promise<void> => {
    if (unlockInFlightRef.current) return unlockInFlightRef.current;
    const promise = (async () => {
      const ctx = ensureContext();
      if (ctx.state !== "running") await ctx.resume();
      // iOS Safari can require an actual (silent) playback within a user gesture.
      const silent = ctx.createBuffer(1, 1, ctx.sampleRate);
      const src = ctx.createBufferSource();
      const gain = ctx.createGain();
      gain.gain.value = 0;
      src.buffer = silent;
      src.connect(gain);
      gain.connect(ctx.destination);
      src.start(0);
      await new Promise<void>((resolve) => {
        src.onended = () => resolve();
      });
    })().finally(() => {
      unlockInFlightRef.current = null;
    });
    unlockInFlightRef.current = promise;
    return promise;
  }, [ensureContext]);

  const playChunk = useCallback((base64PCM: string) => {
    const run = async () => {
      try {
        const binary = atob(base64PCM);
        const byteLen = binary.length;
        const bytes = new Uint8Array(byteLen);
        for (let i = 0; i < byteLen; i++) bytes[i] = binary.charCodeAt(i);
        const numSamples = Math.floor(byteLen / 2);
        const int16 = new Int16Array(bytes.buffer, 0, numSamples);
        const float32 = new Float32Array(numSamples);
        for (let i = 0; i < numSamples; i++) float32[i] = int16[i] / 32768;

        const ctx = ensureContext();
        if (ctx.state !== "running") await unlock();
        const buffer = ctx.createBuffer(1, float32.length, LIVE_SAMPLE_RATE);
        buffer.copyToChannel(float32, 0);
        const source = ctx.createBufferSource();
        source.buffer = buffer;
        source.connect(ctx.destination);
        const start = Math.max(ctx.currentTime, nextStartRef.current);
        source.start(start);
        nextStartRef.current = start + buffer.duration;
        activeSourcesRef.current.add(source);
        source.onended = () => activeSourcesRef.current.delete(source);
      } catch (e) {
        console.warn("PCM play failed:", e);
      }
    };
    run();
  }, [ensureContext, unlock]);

  const stop = useCallback(() => {
    nextStartRef.current = 0;
    activeSourcesRef.current.forEach((s) => {
      try {
        s.stop();
      } catch {
        // already stopped
      }
    });
    activeSourcesRef.current.clear();
  }, []);

  const remainingMs = useCallback((): number => {
    const ctx = ctxRef.current;
    if (!ctx) return 0;
    return Math.max(0, (nextStartRef.current - ctx.currentTime) * 1000);
  }, []);

  const getDebugState = useCallback(() => {
    const ctx = ctxRef.current;
    return {
      hasContext: Boolean(ctx),
      contextState: ctx?.state ?? "none",
    };
  }, []);

  return { playChunk, stop, unlock, remainingMs, getDebugState };
}

export function audioDurationSeconds(chunks: string[]): number {
  let totalSamples = 0;
  for (const b64 of chunks) {
    if (!b64) continue;
    const binary = atob(b64);
    totalSamples += Math.floor(binary.length / 2);
  }
  return totalSamples / LIVE_SAMPLE_RATE;
}

// Build a mono 16-bit PCM WAV blob from base64 PCM chunks.
export function pcmChunksToWavBlob(chunks: string[], sampleRate = LIVE_SAMPLE_RATE): Blob {
  let totalBytes = 0;
  const decoded = chunks.map((b64) => {
    const binary = atob(b64 || "");
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    totalBytes += bytes.length;
    return bytes;
  });

  const headerSize = 44;
  const wav = new Uint8Array(headerSize + totalBytes);
  const view = new DataView(wav.buffer);
  const channels = 1;
  const bitsPerSample = 16;
  const byteRate = sampleRate * channels * (bitsPerSample / 8);
  const blockAlign = channels * (bitsPerSample / 8);

  // RIFF header
  view.setUint32(0, 0x52494646, false); // "RIFF"
  view.setUint32(4, 36 + totalBytes, true);
  view.setUint32(8, 0x57415645, false); // "WAVE"
  view.setUint32(12, 0x666d7420, false); // "fmt "
  view.setUint32(16, 16, true); // PCM fmt chunk size
  view.setUint16(20, 1, true); // PCM format
  view.setUint16(22, channels, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, byteRate, true);
  view.setUint16(32, blockAlign, true);
  view.setUint16(34, bitsPerSample, true);
  view.setUint32(36, 0x64617461, false); // "data"
  view.setUint32(40, totalBytes, true);

  let offset = headerSize;
  for (const part of decoded) {
    wav.set(part, offset);
    offset += part.length;
  }

  return new Blob([wav], { type: "audio/wav" });
}
