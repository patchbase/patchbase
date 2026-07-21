// SPDX-FileCopyrightText: 2026 Configure Labs SRL
// SPDX-License-Identifier: AGPL-3.0-only
import type { WSMessage } from "$lib/types";
import { clearSession } from "$lib/auth/session";

export class WSClient {
  private ws: WebSocket | null = null;
  private token: string;
  private topics: Set<string>;
  private reconnectDelay: number = 1000;
  private handlers: Map<string, Set<(msg: WSMessage) => void>> = new Map();
  private authOk: boolean = false;
  private intentionalClose: boolean = false;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  public connected: boolean = false;
  public onConnectionChange?: (connected: boolean) => void;

  constructor(token: string, topics: string[]) {
    this.token = token;
    this.topics = new Set(topics);
  }

  public getToken(): string {
    return this.token;
  }

  connect(): void {
    if (this.ws || this.intentionalClose) return;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws`;
    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      this.send({ type: "auth", token: this.token });
    };

    this.ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as WSMessage;

        if (msg.type === "auth_ok") {
          this.authOk = true;
          this.connected = true;
          this.onConnectionChange?.(true);
          this.reconnectDelay = 1000; // Reset backoff
          this.send({ type: "subscribe", topics: Array.from(this.topics) });
        } else if (msg.type === "error" && msg.message.includes("unauthorized")) {
          clearSession();
          this.disconnect();
          return;
        }

        const handlers = Array.from(this.handlers.values());
        for (const topicHandlers of handlers) {
          for (const handler of topicHandlers) {
            handler(msg);
          }
        }
      } catch (err) {
        console.error("Failed to parse WS message", err);
      }
    };

    this.ws.onclose = () => {
      this.ws = null;
      this.authOk = false;
      if (this.connected) {
        this.connected = false;
        this.onConnectionChange?.(false);
      }
      if (!this.intentionalClose) {
        this.scheduleReconnect();
      }
    };

    this.ws.onerror = (err) => {
      console.error("WebSocket error", err);
    };
  }

  disconnect(): void {
    this.intentionalClose = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.authOk = false;
    if (this.connected) {
      this.connected = false;
      this.onConnectionChange?.(false);
    }
  }

  on(handler: (msg: WSMessage) => void): () => void {
    const id = crypto.randomUUID();
    const set = new Set<(msg: WSMessage) => void>();
    set.add(handler);
    this.handlers.set(id, set);
    return () => {
      this.handlers.delete(id);
    };
  }

  subscribe(topics: string[]): void {
    for (const t of topics) {
      this.topics.add(t);
    }
    if (this.authOk) {
      this.send({ type: "subscribe", topics });
    }
  }

  unsubscribe(topics: string[]): void {
    for (const t of topics) {
      this.topics.delete(t);
    }
    if (this.authOk) {
      this.send({ type: "unsubscribe", topics });
    }
  }

  private send(payload: any): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(payload));
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) return;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, 5000);
      this.connect();
    }, this.reconnectDelay);
  }
}

export let globalWsClient: WSClient | null = null;

export function initGlobalWsClient(token: string) {
  if (globalWsClient) globalWsClient.disconnect();
  globalWsClient = new WSClient(token, ["hosts", "advisories"]);
  globalWsClient.connect();
}

export function closeGlobalWsClient() {
  if (globalWsClient) {
    globalWsClient.disconnect();
    globalWsClient = null;
  }
}
