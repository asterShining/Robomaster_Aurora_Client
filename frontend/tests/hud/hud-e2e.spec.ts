import { expect, test } from "@playwright/test";
import { fileURLToPath } from "node:url";

const imageFixturePath = fileURLToPath(
  new URL("../../../ca139233357019c6e0526ed173f0eda6.jpg", import.meta.url),
);

async function getLayoutWidgets(page: import("@playwright/test").Page) {
  return page.evaluate(() => {
    const key = Object.keys(window.localStorage).find((item) =>
      item.startsWith("rm-client-hud-layout:v"),
    );
    if (!key) return null;
    const raw = window.localStorage.getItem(key);
    if (!raw) return null;
    return JSON.parse(raw)?.widgets ?? null;
  });
}

async function getShapeItems(page: import("@playwright/test").Page) {
  return page.evaluate(() => {
    const key = Object.keys(window.localStorage).find((item) =>
      item.startsWith("rm-client-hud-shape:v"),
    );
    if (!key) return [];
    const raw = window.localStorage.getItem(key);
    if (!raw) return [];
    return JSON.parse(raw)?.shapes ?? [];
  });
}

async function callDebugBridge(
  page: import("@playwright/test").Page,
  method: "setCombat" | "setMatch" | "setRadar" | "setVideoState" | "reset",
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

test("HUD 核心交互闭环", async ({ page }) => {
  await page.goto("/");

  // What: 验证设置面板可以通过 P 键打开和关闭。
  // Why: 保证 HUD 基础控制链路没有回归。
  await page.keyboard.press("p");
  await expect(
    page.getByRole("dialog", { name: "视频与HUD设置" }),
  ).toBeVisible();

  // What: 验证面板包含视频源和状态卡开关。
  // Why: 确认当前极简面板的核心功能可用。
  await expect(
    page.getByRole("button", { name: "官方视频" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "自定义视频" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "步兵 4 ID 104" })).toBeVisible();
  await page.getByRole("button", { name: "步兵 4 ID 104" }).click();
  await expect(page.locator(".diag-card", { hasText: "机器人身份" })).toContainText(
    "蓝方步兵 4",
  );
  const customVideoButton = page.getByRole("button", { name: "自定义视频" });
  await expect(customVideoButton).toBeDisabled();
  await expect(customVideoButton).toContainText("仅英雄机器人可用");
  await expect(page.locator(".diag-card", { hasText: "自定义链路" })).toContainText(
    "当前身份不使用",
  );
  await page.getByRole("button", { name: "英雄 ID 101" }).click();
  await expect(customVideoButton).toBeEnabled();
  await expect(page.locator(".diag-card", { hasText: "主画面" }).first()).toContainText(
    "官方视频源",
  );

  // What: 通过状态卡开关控制火控组件显隐。
  // Why: 验证即时保存的状态卡开关能正确驱动组件渲染。
  const fireToggle = page
    .locator("label.toggle-row", { hasText: "火控状态卡" })
    .locator("input");
  await expect(page.locator('[data-widget-id="right_fire"]')).toBeVisible();
  await fireToggle.uncheck();
  await expect(page.locator('[data-widget-id="right_fire"]')).toHaveCount(0);
  await fireToggle.check();
  await expect(page.locator('[data-widget-id="right_fire"]')).toBeVisible();

  // What: 关闭设置面板。
  await page
    .getByRole("dialog", { name: "视频与HUD设置" })
    .locator(".close-btn")
    .evaluate((el) => {
      (el as HTMLButtonElement).click();
    });
  await expect(
    page.getByRole("dialog", { name: "视频与HUD设置" }),
  ).not.toBeVisible();

  // What: 通过 localStorage 注入锁定状态，验证锁定后的组件不能拖拽缩放。
  // Why: 防止比赛态误触把关键 HUD 组件拖偏。
  await page.evaluate(() => {
    const key = Object.keys(window.localStorage).find((item) =>
      item.startsWith("rm-client-hud-layout:v"),
    );
    if (!key) return;
    const raw = window.localStorage.getItem(key);
    if (!raw) return;
    const snapshot = JSON.parse(raw);
    if (snapshot.widgets?.left_robot) {
      snapshot.widgets.left_robot.locked = true;
    }
    window.localStorage.setItem(key, JSON.stringify(snapshot));
  });
  await page.reload();

  const editFab = page.getByRole("button", { name: "调整布局" });
  await editFab.click();

  const target = page.locator('[data-widget-id="left_robot"]');
  await expect(target).toBeVisible();

  const beforeLockedWidgets = await getLayoutWidgets(page);
  const beforeLockedScale = beforeLockedWidgets?.left_robot?.scale ?? 1;
  const beforeLockedX = beforeLockedWidgets?.left_robot?.x ?? 0;

  const lockBox = await target.boundingBox();
  if (!lockBox) throw new Error("left_robot bounding box missing (locked)");

  await page.mouse.move(
    lockBox.x + lockBox.width / 2,
    lockBox.y + lockBox.height / 2,
  );
  await page.mouse.wheel(0, -200);
  await page.mouse.move(
    lockBox.x + lockBox.width / 2,
    lockBox.y + lockBox.height / 2,
  );
  await page.mouse.down();
  await page.mouse.move(
    lockBox.x + lockBox.width / 2 + 60,
    lockBox.y + lockBox.height / 2 + 24,
  );
  await page.mouse.up();

  const afterLockedWidgets = await getLayoutWidgets(page);
  expect(afterLockedWidgets?.left_robot?.scale ?? 1).toBe(beforeLockedScale);
  expect(afterLockedWidgets?.left_robot?.x ?? 0).toBe(beforeLockedX);

  // What: 解锁后验证拖拽缩放恢复正常。
  await page.evaluate(() => {
    const key = Object.keys(window.localStorage).find((item) =>
      item.startsWith("rm-client-hud-layout:v"),
    );
    if (!key) return;
    const raw = window.localStorage.getItem(key);
    if (!raw) return;
    const snapshot = JSON.parse(raw);
    if (snapshot.widgets?.left_robot) {
      snapshot.widgets.left_robot.locked = false;
    }
    window.localStorage.setItem(key, JSON.stringify(snapshot));
  });
  await page.reload();
  await editFab.click();

  const beforeWidgets = await getLayoutWidgets(page);
  const beforeScale = beforeWidgets?.left_robot?.scale ?? 1;
  const beforeX = beforeWidgets?.left_robot?.x ?? 0;
  const box = await target.boundingBox();
  if (!box) throw new Error("left_robot bounding box missing");

  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.wheel(0, -200);
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  await page.mouse.move(
    box.x + box.width / 2 + 60,
    box.y + box.height / 2 + 24,
  );
  await page.mouse.up();

  const afterWidgets = await getLayoutWidgets(page);
  const afterScale = afterWidgets?.left_robot?.scale ?? 1;
  const afterX = afterWidgets?.left_robot?.x ?? 0;
  expect(afterScale).toBeGreaterThan(beforeScale);
  expect(afterScale).toBeGreaterThanOrEqual(0.7);
  expect(afterScale).toBeLessThanOrEqual(1.5);
  expect(afterX).not.toBe(beforeX);

  await expect(
    page.locator('[data-widget-id="top_red_base_hp"]'),
  ).toBeVisible();
  await expect(page.locator('[data-widget-id="top_red_team"]')).toBeVisible();
  await expect(page.locator('[data-widget-id="top_score"]')).toBeVisible();
  await expect(page.locator('[data-widget-id="top_blue_team"]')).toBeVisible();
  await expect(
    page.locator('[data-widget-id="top_blue_base_hp"]'),
  ).toBeVisible();
  await expect(page.locator('[data-widget-id="right_fire"]')).toBeVisible();
  await expect(page.locator('[data-widget-id="left_robot"]')).toBeVisible();
});

test("O 键一键买弹遵守预设值与可购买门控", async ({ page }) => {
  await page.addInitScript(() => {
    const quickActionCalls: Array<{ action: string; value: number }> = [];
    const globalWindow = window as Window & {
      __RM_QUICK_ACTION_CALLS__?: Array<{ action: string; value: number }>;
      go?: {
        main?: {
          App?: {
            TriggerQuickAction?: (
              action: string,
              value: number,
            ) => Promise<void>;
          };
        };
      };
    };

    globalWindow.__RM_QUICK_ACTION_CALLS__ = quickActionCalls;
    globalWindow.go = globalWindow.go ?? {};
    globalWindow.go.main = globalWindow.go.main ?? {};
    globalWindow.go.main.App = globalWindow.go.main.App ?? {};
    globalWindow.go.main.App.TriggerQuickAction = async (
      action: string,
      value: number,
    ) => {
      quickActionCalls.push({ action, value });
    };
  });

  await page.goto("/");

  // What: 在允许远程买弹时按下 O 键。
  // Why: 验证只触发一次后端快捷动作，并使用默认预设值。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setCombat", {
    canRemoteAmmo: true,
  });
  await page.keyboard.press("o");
  await expect(page.locator(".capture-toggle small")).toContainText(
    "已发送买弹",
  );
  await expect
    .poll(async () => {
      return page.evaluate(() => {
        return (
          (
            window as Window & {
              __RM_QUICK_ACTION_CALLS__?: Array<{
                action: string;
                value: number;
              }>;
            }
          ).__RM_QUICK_ACTION_CALLS__ ?? []
        );
      });
    })
    .toEqual([{ action: "buy_ammo", value: 20 }]);

  // What: 设置面板打开时屏蔽一键买弹。
  // Why: 防止调参过程中误触 O 键直接下发补给命令。
  await page.keyboard.press("p");
  await expect(
    page.getByRole("dialog", { name: "视频与HUD设置" }),
  ).toBeVisible();
  await page.keyboard.press("o");
  await expect
    .poll(async () => {
      return page.evaluate(() => {
        return (
          (
            window as Window & {
              __RM_QUICK_ACTION_CALLS__?: Array<{
                action: string;
                value: number;
              }>;
            }
          ).__RM_QUICK_ACTION_CALLS__?.length ?? 0
        );
      });
    })
    .toBe(1);
  await page
    .getByRole("dialog", { name: "视频与HUD设置" })
    .locator(".close-btn")
    .evaluate((el) => {
      (el as HTMLButtonElement).click();
    });
  await expect(
    page.getByRole("dialog", { name: "视频与HUD设置" }),
  ).not.toBeVisible();

  // What: 在不可远程买弹时再次按下 O 键。
  // Why: 验证前端门控会拦截动作，只给轻提示而不继续发送命令。
  await callDebugBridge(page, "setCombat", {
    canRemoteAmmo: false,
  });
  await page.keyboard.press("o");
  await expect(page.locator(".capture-toggle small")).toContainText(
    "当前不可远程买弹",
  );
  await expect
    .poll(async () => {
      return page.evaluate(() => {
        return (
          (
            window as Window & {
              __RM_QUICK_ACTION_CALLS__?: Array<{
                action: string;
                value: number;
              }>;
            }
          ).__RM_QUICK_ACTION_CALLS__?.length ?? 0
        );
      });
    })
    .toBe(1);
});

test("血量补丁对齐后不会用缺失字段覆盖真实上限", async ({ page }) => {
  await page.goto("/");

  // What: 先注入一组完整的基地、本机和编组血量数据。
  // Why: 这条用例要锁住"真实 maxHp 先到达后，后续缺字段 patch 不能把它洗掉"的回归面。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setMatch", {
    globalStatusReady: true,
    red: {
      baseHp: 4100,
      baseMaxHp: 5000,
      units: [{ robotId: 1, hp: 200, maxHp: 450, online: true }],
    },
    blue: {
      baseHp: 3900,
      baseMaxHp: 5000,
      units: [{ robotId: 1, hp: 320, maxHp: 400, online: true }],
    },
  });
  await callDebugBridge(page, "setCombat", {
    hp: 123,
    maxHp: 250,
    origin: "backend",
  });

  await expect(
    page.locator('[data-widget-id="top_red_base_hp"]'),
  ).toContainText("4100 / 5000");
  await expect(
    page.locator('[data-widget-id="top_blue_base_hp"]'),
  ).toContainText("3900 / 5000");
  await expect(page.locator('[data-widget-id="top_red_team"]')).toContainText(
    "200 / 450",
  );
  await expect(page.locator('[data-widget-id="top_blue_team"]')).toContainText(
    "320 / 400",
  );
  await expect(page.locator('[data-widget-id="left_robot"]')).toContainText(
    "123 / 250",
  );

  // What: 再注入一个故意缺少 maxHp 的 patch。
  // Why: 后端静态状态可能低频到达，这里必须证明前端会保留上一轮真实 maxHp，而不是退回默认值。
  await callDebugBridge(page, "setMatch", {
    red: {
      units: [{ robotId: 1, hp: 180 }],
    },
  });
  await callDebugBridge(page, "setCombat", {
    hp: 118,
  });

  await expect(page.locator('[data-widget-id="top_red_team"]')).toContainText(
    "180 / 450",
  );
  await expect(page.locator('[data-widget-id="left_robot"]')).toContainText(
    "118 / 250",
  );
});

test("满血血条应完全填充，扣血后才显示缺口", async ({ page }) => {
  await page.goto("/");

  // What: 先注入一组明确的满血基地与满血编组数据。
  // Why: 这条用例要锁住"HP 等于 maxHp 时，填充必须铺满整条轨道"的视觉约束。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setCombat", {
    hp: 250,
    maxHp: 250,
    origin: "backend",
  });
  await callDebugBridge(page, "setMatch", {
    globalStatusReady: true,
    red: {
      baseHp: 5000,
      baseMaxHp: 5000,
      units: [{ robotId: 1, hp: 450, maxHp: 450, online: true }],
    },
  });

  const redBaseTrack = page
    .locator('[data-widget-id="top_red_base_hp"] .track')
    .first();
  const redBaseFill = page
    .locator('[data-widget-id="top_red_base_hp"] .track > i')
    .first();
  const redUnitCard = page
    .locator('[data-widget-id="top_red_team"] .team-robot-card')
    .first();
  const redUnitTrack = redUnitCard.locator(".track");
  const redUnitFill = redUnitCard.locator(".track > i");
  const robotHpTrack = page
    .locator('[data-widget-id="left_robot"] [data-testid="hud-robot-hp-track"]')
    .first();
  const robotHpFill = page
    .locator('[data-widget-id="left_robot"] [data-testid="hud-robot-hp-fill"]')
    .first();

  // What: 满血时校验填充宽度几乎等于轨道宽度。
  // Why: 若两者仍稳定差 2px 左右，就说明视觉上依旧会残留"明明满血却还有缺口"的问题。
  await expect
    .poll(async () => {
      const trackBox = await redBaseTrack.boundingBox();
      const fillBox = await redBaseFill.boundingBox();
      if (!trackBox || !fillBox) return Number.POSITIVE_INFINITY;
      return Math.abs(trackBox.width - fillBox.width);
    })
    .toBeLessThanOrEqual(2);

  await expect
    .poll(async () => {
      const trackBox = await redUnitTrack.boundingBox();
      const fillBox = await redUnitFill.boundingBox();
      if (!trackBox || !fillBox) return Number.POSITIVE_INFINITY;
      return Math.abs(trackBox.width - fillBox.width);
    })
    .toBeLessThanOrEqual(2);

  await expect
    .poll(async () => {
      const trackBox = await robotHpTrack.boundingBox();
      const fillBox = await robotHpFill.boundingBox();
      if (!trackBox || !fillBox) return Number.POSITIVE_INFINITY;
      return Math.abs(trackBox.width - fillBox.width);
    })
    .toBeLessThanOrEqual(2);

  // What: 再把基地与机器人都扣血。
  // Why: 需要同时证明"缺口"只会在非满血时出现，而不是所有状态都被强制铺满。
  await callDebugBridge(page, "setMatch", {
    red: {
      baseHp: 4200,
      units: [{ robotId: 1, hp: 300 }],
    },
  });
  await callDebugBridge(page, "setCombat", {
    hp: 120,
  });

  await expect
    .poll(async () => {
      const trackBox = await redBaseTrack.boundingBox();
      const fillBox = await redBaseFill.boundingBox();
      if (!trackBox || !fillBox) return 0;
      return trackBox.width - fillBox.width;
    })
    .toBeGreaterThan(4);

  await expect
    .poll(async () => {
      const trackBox = await redUnitTrack.boundingBox();
      const fillBox = await redUnitFill.boundingBox();
      if (!trackBox || !fillBox) return 0;
      return trackBox.width - fillBox.width;
    })
    .toBeGreaterThan(4);

  await expect
    .poll(async () => {
      const trackBox = await robotHpTrack.boundingBox();
      const fillBox = await robotHpFill.boundingBox();
      if (!trackBox || !fillBox) return 0;
      return trackBox.width - fillBox.width;
    })
    .toBeGreaterThan(4);
});

test("没有真实 combat-state 时本车血量区显示未接入", async ({ page }) => {
  await page.goto("/");

  // What: 只注入视频状态，不注入真实 combat-state。
  // Why: 严格真实模式下，本车生命值区域必须明确告诉用户"未接入"，不能再把 simulated 默认值当成真实满血。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setVideoState", {
    backend_connected: true,
    control_link_connected: true,
    video_connected: true,
    display_state: "live",
    latency_ms: 42,
    message: "原生低延迟视频链路正常",
  });

  const robotCard = page.locator('[data-widget-id="left_robot"]');
  const robotHpTrack = robotCard.locator('[data-testid="hud-robot-hp-track"]');
  const robotHpFill = robotCard.locator('[data-testid="hud-robot-hp-fill"]');

  await expect(robotCard).toContainText("未接入");
  await expect(robotCard).toContainText("等待真实 combat-state");

  await expect
    .poll(async () => {
      const trackBox = await robotHpTrack.boundingBox();
      const fillBox = await robotHpFill.boundingBox();
      if (!trackBox || !fillBox) return Number.POSITIVE_INFINITY;
      return fillBox.width;
    })
    .toBeLessThanOrEqual(1);
});

test("顶部中条严格显示真实比分与比赛时间", async ({ page }) => {
  await page.goto("/");

  // What: 直接注入一组完整的官方比赛状态。
  // Why: 这条用例锁住顶部中条对 GameStatus 的真实字段映射，避免以后又回退到 round/economy 假字段。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setMatch", {
    gameStatusReady: true,
    currentRound: 2,
    totalRounds: 5,
    redScore: 3,
    blueScore: 1,
    currentStage: 4,
    stageCountdownSec: 95,
    stageElapsedSec: 265,
    isPaused: true,
  });

  const scoreCard = page.locator('[data-widget-id="top_score"]');
  await expect(scoreCard).toContainText("红方");
  await expect(scoreCard).toContainText("蓝方");
  await expect(scoreCard).toContainText("已暂停");
  await expect(page.getByTestId("hud-match-round")).toContainText(
    "第 2 / 5 局",
  );
  await expect(page.getByTestId("hud-match-stage")).toContainText("比赛中");
  await expect(page.getByTestId("hud-match-countdown")).toContainText(
    "剩余 01:35",
  );
  await expect(page.getByTestId("hud-match-elapsed")).toContainText(
    "已进行 04:25",
  );
});

test("没有真实 combat-state 时火控与本车组件显示未接入", async ({
  page,
}) => {
  await page.goto("/");

  // What: 只保留视频链路在线，不注入真实火控。
  // Why: 用户要求无真实来源时明确显示未接入，不能再保留 simulated 或默认值展示。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setVideoState", {
    backend_connected: true,
    control_link_connected: true,
    video_connected: true,
    display_state: "live",
    latency_ms: 31,
    message: "原生低延迟视频链路正常",
  });

  await expect(page.locator('[data-widget-id="right_fire"]')).toContainText(
    "未接入",
  );
  await expect(page.locator('[data-widget-id="right_fire"]')).toContainText(
    "等待真实 combat-state",
  );
  await expect(page.locator('[data-widget-id="left_robot"]')).toContainText(
    "未接入",
  );
  await expect(page.locator('[data-widget-id="left_robot"]')).toContainText(
    "等待真实 combat-state",
  );
});

test("状态卡标签与链路诊断字段对齐", async ({ page }) => {
  await page.goto("/");

  // What: 构造一组"后端在线、控制在线、图传离线、延迟偏高"的连接态。
  // Why: 锁住本车状态卡和链路诊断卡与真实后端字段的对应关系。
  await callDebugBridge(page, "reset");
  await callDebugBridge(page, "setCombat", {
    hp: 320,
    maxHp: 450,
  });
  await callDebugBridge(page, "setMatch", {
    gameStatusReady: true,
    currentRound: 1,
    totalRounds: 3,
    redScore: 0,
    blueScore: 0,
    currentStage: 4,
    stageCountdownSec: 120,
    stageElapsedSec: 60,
    red: {
      baseHp: 4300,
    },
    blue: {
      baseHp: 4800,
    },
  });
  await callDebugBridge(page, "setVideoState", {
    backend_connected: true,
    control_link_connected: true,
    video_connected: false,
    display_state: "stalled",
    latency_ms: 286,
    message: "视频状态延迟",
  });

  const robotCard = page.locator('[data-widget-id="left_robot"]');
  const moduleCard = page.locator('[data-widget-id="left_modules"]');

  // 本车状态卡应显示当前视频源、战斗态、显示状态和延迟
  await expect(robotCard).toContainText("当前视频源");
  await expect(robotCard).toContainText("战斗态");
  await expect(robotCard).toContainText("显示状态");
  await expect(robotCard).toContainText("当前延迟");
  await expect(robotCard).toContainText("已断开");
  await expect(robotCard).toContainText("286 ms");

  // 链路诊断卡应显示双视频源和链路状态
  await expect(moduleCard).toContainText("当前源");
  await expect(moduleCard).toContainText("官方源");
  await expect(moduleCard).toContainText("自定义源");
  await expect(moduleCard).toContainText("MQTT链路");
  await expect(moduleCard).toContainText("显示状态");
  await expect(moduleCard).toContainText("当前延迟");
  await expect(moduleCard).toContainText("286 ms");
  await expect(moduleCard).toContainText("已断开");
});
