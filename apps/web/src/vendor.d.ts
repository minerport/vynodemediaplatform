declare module "*.mjs" {
  type Handler = (...args: any[]) => void;
  export default class Hls {
    static isSupported(): boolean;
    static Events: { MANIFEST_PARSED: string; ERROR: string };
    constructor(config?: { enableWorker?: boolean });
    loadSource(url: string): void;
    attachMedia(media: HTMLMediaElement): void;
    on(event: string, handler: Handler): void;
    destroy(): void;
  }
}
