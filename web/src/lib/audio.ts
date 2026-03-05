"use client";

import { useCallback, useRef } from "react";

export const LIVE_SAMPLE_RATE = 24000;

export function usePCMPlayer() {
  const ctxRef = useRef<AudioContext | null>(null);
  const nextStartRef = useRef(0);
  const activeSourcesRef = useRef<Set<AudioBufferSourceNode>>(new Set());

  const playChunk = useCallback((base64PCM: string) => {
    try {
      const binary = atob(base64PCM);
      const byteLen = binary.length;
      const bytes = new Uint8Array(byteLen);
      for (let i = 0; i < byteLen; i++) bytes[i] = binary.charCodeAt(i);
      const numSamples = Math.floor(byteLen / 2);
      const int16 = new Int16Array(bytes.buffer, 0, numSamples);
      const float32 = new Float32Array(numSamples);
      for (let i = 0; i < numSamples; i++) float32[i] = int16[i] / 32768;

      let ctx = ctxRef.current;
      if (!ctx) {
        const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
        ctx = new Ctx();
        ctxRef.current = ctx;
        nextStartRef.current = ctx.currentTime;
      }
      if (ctx.state === "suspended") ctx.resume();
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
  }, []);

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

  const unlock = useCallback(() => {
    if (ctxRef.current) {
      if (ctxRef.current.state === "suspended") ctxRef.current.resume();
      return;
    }
    const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
    const ctx = new Ctx();
    ctxRef.current = ctx;
    nextStartRef.current = ctx.currentTime;
    if (ctx.state === "suspended") ctx.resume();
  }, []);

  return { playChunk, stop, unlock, remainingMs };
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
