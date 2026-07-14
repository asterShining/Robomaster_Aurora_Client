// env.d.ts
/// <reference types="vite/client" />

interface Window {
  go: {
    main: {
      App: {
        PushKeyboardMouse(
            x: number, y: number, z: number, l: boolean, r: boolean, m: boolean,
            kb: number): Promise<void>;
        TriggerQuickAction(action: string, value: number): Promise<void>;
        SaveClientConfig(configJSON: string): Promise<void>;
        GetClientConfig(): Promise<string>;
        SetActiveVideoSource(source: string): Promise<void>;
        SetMQTTRobotIdentity(identity: string): Promise<void>;
        SetMQTTHeroIdentity(identity: string): Promise<void>;
      }
    }
  }
}
