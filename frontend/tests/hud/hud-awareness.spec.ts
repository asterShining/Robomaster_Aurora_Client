import { expect, test } from "@playwright/test";

async function callDebugBridge(
  page: import("@playwright/test").Page,
  method:
    | "setCombat"
    | "setMatch"
    | "setRadar"
    | "setVideoState"
    | "setVision"
    | "reset",
  payload?: Record<string, unknown>,
) {
  await page.evaluate(
    ({ method, payload }) => {
      const bridge = (window as Window & { __RM_DEBUG_HUD__?: unknown })
        .__RM_DEBUG_HUD__ as
        | {
            setCombat: (patch: Record<string, unknown>) => void;
            setMatch: (patch: Record<string, unknown>) => void;
            setRadar: (patch: Record<string, unknown>) => void;
            setVideoState: (patch: Record<string, unknown>) => void;
            setVision: (patch: Record<string, unknown>) => void;
            reset: () => void;
          }
        | undefined;

      if (!bridge) throw new Error("debug bridge missing");
      if (method === "reset") {
        bridge.reset();
        return;
      }
      bridge[method](payload ?? {});
    },
    { method, payload },
  );
}

test("官方视频断流遮幕可出现并恢复", async ({ page }) => {
  await page.goto("/");
  await page.setViewportSize({ width: 1365, height: 768 });

  // What: 先把视频状态显式切到断流态。
  // Why: 这条用例只验证遮幕与恢复逻辑，必须绕过真实后端时序，直接锁定最关键的前端降级行为。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setVideoState", {
    backend_connected: true,
    control_link_connected: true,
    video_connected: false,
    display_state: "stalled",
    latency_ms: 932,
    present_fps: 0,
    decoder_resets: 2,
    message: "官方视频已断开，等待恢复",
  });

  await expect(page.getByTestId("video-overlay")).toContainText(
    "官方视频已断开",
  );
  await expect(page.getByTestId("video-overlay")).toContainText(
    "官方视频已断开，等待恢复",
  );
  await expect(page.locator(".app-video-curtain--blocked")).toBeVisible();

  // What: 再把链路恢复为 live。
  // Why: 断流提示必须能自动退出，否则战场画面恢复后仍会被遮幕挡住。
  await callDebugBridge(page, "setVideoState", {
    backend_connected: true,
    control_link_connected: true,
    video_connected: true,
    display_state: "live",
    decoder_fps: 59.8,
    present_fps: 59.4,
    latency_ms: 28,
    decoder_resets: 2,
    message: "原生低延迟视频链路正常",
  });

  await expect(page.getByTestId("video-overlay")).toHaveCount(0);
  await expect(page.locator(".app-video-curtain--blocked")).toHaveCount(0);
});

test("自定义视频等待态使用纯黑幕并保留 HUD", async ({ page }) => {
  await page.goto("/");
  await page.setViewportSize({ width: 1365, height: 768 });

  // What: 直接注入 custom 源的等待态。
  // Why: 用户当前最在意的是切到自定义源后不能再透出桌面，因此这里要单独锁住 custom 源的黑幕样式与标题。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setVideoState", {
    backend_connected: false,
    control_link_connected: true,
    video_connected: false,
    active_source: "custom",
    official_available: true,
    custom_available: false,
    display_state: "waiting_source",
    message: "等待 0x0310 自定义视频数据",
  });

  const curtain = page.locator(".app-video-curtain--blocked");
  await expect(page.getByTestId("video-overlay")).toContainText("等待自定义视频");
  await expect(page.getByTestId("video-overlay")).toContainText("等待 0x0310 自定义视频数据");
  await expect(curtain).toBeVisible();
  await expect(curtain).toHaveCSS("background-color", "rgb(0, 0, 0)");
});

test("默认 official 主画面时保留 custom PiP 槽位", async ({ page }) => {
  await page.goto("/");
  await page.setViewportSize({ width: 1365, height: 768 });

  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setVideoState", {
    backend_connected: true,
    control_link_connected: true,
    video_connected: true,
    active_source: "official",
    pip_source: "custom",
    official_available: true,
    custom_available: true,
    official_video_connected: true,
    custom_video_connected: true,
    display_state: "live",
    official_display_state: "live",
    custom_display_state: "live",
    message: "主画面正常",
    official_message: "主画面正常",
    custom_message: "PiP 正常",
  });

  const pipSlot = page.getByTestId("pip-slot");
  await expect(pipSlot).toContainText("PiP · 自定义");
  await expect(pipSlot).toContainText("在线");
  await expect(page.getByTestId("video-overlay")).toHaveCount(0);
});

test("切主画面到 custom 后 official 进入 PiP 黑底占位", async ({ page }) => {
  await page.goto("/");
  await page.setViewportSize({ width: 1365, height: 768 });

  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setVideoState", {
    backend_connected: true,
    control_link_connected: true,
    video_connected: true,
    active_source: "custom",
    pip_source: "official",
    official_available: false,
    custom_available: true,
    official_video_connected: false,
    custom_video_connected: true,
    display_state: "live",
    official_display_state: "stalled",
    custom_display_state: "live",
    message: "自定义主画面正常",
    official_message: "官方 H.265 视频源已中断，最近未收到新的 UDP 视频包",
    custom_message: "自定义主画面正常",
  });

  const pipSlot = page.getByTestId("pip-slot");
  await expect(pipSlot).toContainText("PiP · 官方");
  await expect(pipSlot).toContainText("官方 PiP 已断开");
  await expect(pipSlot).toContainText(
    "官方 H.265 视频源已中断，最近未收到新的 UDP 视频包",
  );
  await expect(pipSlot).toHaveClass(/is-offline/);
  await expect(page.getByTestId("video-overlay")).toHaveCount(0);
});

test("态势增强器关键提示闭环", async ({ page }) => {
  await page.goto("/");
  await page.setViewportSize({ width: 1365, height: 768 });

  // What: 通过调试桥直接注入真实语义的比赛态、战斗态和目标态。
  // Why: 测试环境没有真实 Wails runtime 事件桥，必须绕过后端依赖验证 HUD 增强器。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setMatch", {
    gameStatusReady: true,
    currentRound: 2,
    totalRounds: 5,
    redScore: 1,
    blueScore: 2,
    currentStage: 4,
    stageCountdownSec: 83,
    stageElapsedSec: 217,
  });
  await callDebugBridge(page, "setCombat", {
    hp: 28,
    maxHp: 200,
    ammo: 12,
    maxAmmo: 120,
    heat: 20,
    maxHeat: 100,
    bulletSpeed: 28.6,
  });
  await callDebugBridge(page, "setRadar", {
    rangeM: 15,
    sweepDeg: 96,
    source: "backend",
    contacts: [
      {
        id: "rad-1",
        x: 6.2,
        y: 1.8,
        velocity: 2.4,
        side: "enemy",
        confidence: 0.88,
        updatedAt: Date.now(),
      },
    ],
  });
  await callDebugBridge(page, "setVision", {
    targets: [
      {
        id: "vis-1",
        xNorm: 0.64,
        yNorm: 0.44,
        confidence: 0.93,
        side: "enemy",
        armorLabel: "3号",
        updatedAt: Date.now(),
      },
    ],
  });

  await expect(page.getByTestId("hud-awareness-alert")).toContainText(
    "血量危险",
  );
  await expect(page.getByTestId("hud-awareness-threat")).toContainText("VIS");
  await expect(page.getByTestId("hud-awareness-vignette")).toBeVisible();
  await expect(page.locator(".robot-replica")).toHaveClass(/alert-critical/);
  await expect(
    page.locator('[data-testid=\"ammo-segment\"].is-active'),
  ).toHaveCount(2);
  await expect(page.getByTestId("hud-match-round")).toContainText(
    "第 2 / 5 局",
  );
  await expect(page.getByTestId("hud-match-stage")).toContainText("比赛中");

  // What: 第二次注入高热量场景。
  // Why: 验证准星和火控卡能同步进入“即将过热”态，而不是只有中心弹窗变化。
  await callDebugBridge(page, "setCombat", {
    hp: 28,
    maxHp: 200,
    ammo: 12,
    maxAmmo: 120,
    heat: 97,
    maxHeat: 100,
    bulletSpeed: 28.6,
  });

  await expect(page.getByTestId("hud-crosshair")).toHaveClass(/imminent-heat/);
  await expect(page.locator(".heat-alert")).toContainText("0.5s 后过热");
  await expect(page.getByTestId("hud-radar-focus")).toContainText("VIS");
});

test("编辑态隐藏态势增强器叠层", async ({ page }) => {
  await page.goto("/");
  await page.setViewportSize({ width: 1365, height: 768 });

  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setCombat", {
    hp: 20,
    maxHp: 200,
    ammo: 18,
    maxAmmo: 120,
    heat: 30,
    maxHeat: 100,
    bulletSpeed: 26.5,
  });
  await callDebugBridge(page, "setRadar", {
    rangeM: 15,
    sweepDeg: 45,
    source: "backend",
    contacts: [
      {
        id: "rad-2",
        x: 4.8,
        y: -3.6,
        velocity: 1.9,
        side: "enemy",
        confidence: 0.81,
        updatedAt: Date.now(),
      },
    ],
  });
  await callDebugBridge(page, "setVision", {
    targets: [
      {
        id: "vis-hide-1",
        xNorm: 0.55,
        yNorm: 0.48,
        confidence: 0.86,
        side: "enemy",
        armorLabel: "4号",
        updatedAt: Date.now(),
      },
    ],
  });

  await expect(page.getByTestId("hud-awareness-vignette")).toBeVisible();
  await expect(page.getByTestId("hud-awareness-armor")).toBeVisible();
  await page.keyboard.press("u");
  await expect(page.locator(".editor-dock")).toBeVisible();
  await expect(page.getByTestId("hud-awareness-vignette")).toHaveCount(0);
  await expect(page.getByTestId("hud-awareness-alert")).toHaveCount(0);
  await expect(page.getByTestId("hud-awareness-armor")).toHaveCount(0);
  await expect(page.getByTestId("hud-awareness-threat")).toHaveCount(0);
});

test("电容与比赛时间压力接入同一套增强器规则", async ({ page }) => {
  await page.goto("/");
  await page.setViewportSize({ width: 1365, height: 768 });

  // What: 注入低电容与真实终局时间场景。
  // Why: 这两类数据来自不同状态源，必须验证它们能被同一套增强器规则稳定消费。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setMatch", {
    gameStatusReady: true,
    currentRound: 3,
    totalRounds: 5,
    redScore: 4,
    blueScore: 3,
    currentStage: 4,
    stageCountdownSec: 18,
    stageElapsedSec: 342,
  });
  await callDebugBridge(page, "setCombat", {
    hp: 132,
    maxHp: 200,
    ammo: 54,
    maxAmmo: 120,
    heat: 42,
    maxHeat: 100,
    bulletSpeed: 27.4,
    capacitorPct: 9,
  });
  await callDebugBridge(page, "setVision", {
    targets: [
      {
        id: "vis-cap-1",
        xNorm: 0.62,
        yNorm: 0.46,
        confidence: 0.94,
        side: "enemy",
        armorLabel: "英雄",
        updatedAt: Date.now(),
      },
    ],
  });

  await expect(page.getByTestId("hud-capacitor-row")).toHaveClass(
    /level-critical/,
  );
  await expect(page.getByTestId("hud-awareness-armor")).toBeVisible();
  await expect(page.getByTestId("hud-match-countdown")).toHaveClass(
    /level-critical/,
  );
});

test("没有真实 GameStatus 时顶部中条显示未接入且不会误报时间压力", async ({
  page,
}) => {
  await page.goto("/");
  await page.setViewportSize({ width: 1365, height: 768 });

  // What: 只注入双方血量，不注入 GameStatus。
  // Why: 顶部中条必须明确显示“未接入”，并且不能把默认 0 秒误判成终局读秒。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setMatch", {
    red: {
      baseHp: 4200,
      baseMaxHp: 5000,
    },
    blue: {
      baseHp: 4100,
      baseMaxHp: 5000,
    },
  });

  await expect(page.getByTestId("hud-match-unavailable")).toContainText(
    "比赛信息未接入",
  );
  await expect(page.getByTestId("hud-match-unavailable")).toContainText(
    "等待真实 GameStatus",
  );
  await expect(page.getByTestId("hud-awareness-alert")).toHaveCount(0);
});
