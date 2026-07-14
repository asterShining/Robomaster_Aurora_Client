<template>
  <main
    class="app-stage"
    :class="[
      videoState.pip_source === 'custom'
        ? 'app-stage--pip-custom'
        : 'app-stage--pip-official',
    ]"
  >
    <!-- What: 当前激活源不可用时叠加重幕布。Why: 现在双源切换是真实后端行为，必须明确告诉用户“当前源没有出画”。 -->
    <div
      v-if="showBlockingVideoOverlay"
      class="app-video-curtain"
      :class="{ 'app-video-curtain--pip-cutout': pipShouldPunchThrough }"
      aria-hidden="true"
    >
      <span class="app-video-curtain__piece app-video-curtain__piece--top"></span>
      <span class="app-video-curtain__piece app-video-curtain__piece--left"></span>
      <span class="app-video-curtain__piece app-video-curtain__piece--right"></span>
      <span class="app-video-curtain__piece app-video-curtain__piece--bottom"></span>
    </div>

    <section
      v-if="showBlockingVideoOverlay"
      class="app-video-overlay app-video-overlay--underlay hud-realtime"
      data-testid="video-overlay"
    >
      <p class="fallback-title">{{ videoStatusTitle }}</p>
      <p class="fallback-sub">{{ videoState.message }}</p>
      <p v-if="videoStatusMeta" class="fallback-meta">{{ videoStatusMeta }}</p>
    </section>

    <section
      class="app-pip-slot hud-realtime"
      :class="{
        'is-live': pipVideoLive,
        'is-custom': videoState.pip_source === 'custom',
        'is-pass-through': pipShouldPunchThrough,
        'is-offline': !pipVideoLive,
      }"
      data-testid="pip-slot"
    >
      <header class="app-pip-slot__head">
        <span class="app-pip-slot__badge">PiP · {{ pipSourceLabel }}</span>
        <span class="app-pip-slot__state">{{ pipStateLabel }}</span>
      </header>

      <div v-if="!pipVideoLive && !pipShouldPunchThrough" class="app-pip-slot__empty">
        <p class="app-pip-slot__title">{{ pipStatusTitle }}</p>
        <p class="app-pip-slot__sub">{{ pipMessage }}</p>
      </div>
    </section>

    <section v-if="runtimeBridgeError" class="app-runtime-status hud-realtime">
      <p class="fallback-title">运行时桥异常</p>
      <p class="fallback-sub">{{ videoState.message }}</p>
    </section>

    <!-- What: 扫描线层只承担轻微氛围。Why: 功能精简后更需要靠少量固定装饰维持整体 HUD 质感。 -->
    <div class="app-scanlines" aria-hidden="true"></div>

    <HudLayout />
  </main>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import HudLayout from "./components/HudLayout.vue";
import { useRobotDataStore } from "./store/robotData";
import { useUiPanelStore } from "./store/uiPanel";
import type {
  BuffState,
  CombatData,
  DeployModeStatusState,
  GlobalLogisticsState,
  GlobalSpecialMechanismState,
  MatchState,
  RadarData,
  RefereeEventEntry,
  RobotModuleStatusState,
  RobotStaticStatusState,
  TeamRobotState,
  TeamRole,
  VideoDisplayState,
  VideoSource,
  VisionData,
} from "./types";

interface RawVideoStatePayload {
  backend_connected: boolean;
  control_link_connected: boolean;
  video_connected: boolean;
  active_source: VideoSource;
  pip_source: VideoSource;
  official_available: boolean;
  custom_available: boolean;
  official_video_connected: boolean;
  custom_video_connected: boolean;
  display_state: VideoDisplayState;
  official_display_state: VideoDisplayState;
  custom_display_state: VideoDisplayState;
  decoder_fps: number;
  present_fps: number;
  latency_ms: number;
  official_latency_ms: number;
  custom_latency_ms: number;
  official_packet_rate_hz: number;
  official_frame_rate_hz: number;
  official_drop_rate: number;
  custom_block_rate_hz: number;
  custom_au_fps: number;
  custom_drop_rate: number;
  custom_corrupt_rate: number;
  custom_h264_nal_total: number;
  custom_h264_idr: number;
  custom_h264_sps: number;
  custom_h264_pps: number;
  custom_h264_no_start_code: number;
  custom_h264_synced: boolean;
  udp_drop_frames: number;
  decode_drop_frames: number;
  corrupt_frames: number;
  decoder_resets: number;
  header_order: string;
  message: string;
  official_message: string;
  custom_message: string;
  updated_at: number;
}

interface RawMatchUnitPayload {
  robot_id?: number;
  role?: string;
  hp?: number;
  max_hp?: number;
  avatar_key?: string;
  online?: boolean;
}

interface RawMatchSidePayload {
  base_hp?: number;
  base_max_hp?: number;
  base_status?: number;
  base_shield?: number;
  outpost_hp?: number;
  outpost_status?: number;
  total_damage?: number;
  units?: RawMatchUnitPayload[];
}

interface RawMatchStatePayload {
  red?: RawMatchSidePayload;
  blue?: RawMatchSidePayload;
  global_status_ready?: boolean;
  game_status_ready?: boolean;
  current_round?: number;
  total_rounds?: number;
  red_score?: number;
  blue_score?: number;
  current_stage?: number;
  stage_countdown_sec?: number;
  stage_elapsed_sec?: number;
  is_paused?: boolean;
  updated_at?: number;
}

interface RawCombatStatePayload {
  hp?: number;
  max_hp?: number;
  ammo?: number;
  max_ammo?: number;
  heat?: number;
  max_heat?: number;
  last_projectile_fire_rate?: number;
  current_chassis_energy?: number;
  current_buffer_energy?: number;
  current_experience?: number;
  experience_for_upgrade?: number;
  total_projectiles_fired?: number;
  is_out_of_combat?: boolean;
  out_of_combat_countdown_sec?: number;
  can_remote_heal?: boolean;
  bullet_speed?: number;
  capacitor_pct?: number;
  can_remote_ammo?: boolean;
  updated_at?: number;
}

interface RawGlobalLogisticsPayload {
  remaining_economy?: number;
  total_economy_obtained?: number;
  tech_level?: number;
  encryption_level?: number;
  updated_at?: number;
}

interface RawSpecialMechanismPayload {
  mechanisms?: Array<{ mechanism_id?: number; time_sec?: number }>;
  updated_at?: number;
}

interface RawRefereeEventPayload {
  event_id?: number;
  param?: string;
  updated_at?: number;
}

interface RawRobotStaticStatusPayload {
  connection_state?: number;
  field_state?: number;
  alive_state?: number;
  robot_id?: number;
  robot_type?: number;
  performance_system_shooter?: number;
  performance_system_chassis?: number;
  level?: number;
  max_health?: number;
  max_heat?: number;
  heat_cooldown_rate?: number;
  max_power?: number;
  max_buffer_energy?: number;
  max_chassis_energy?: number;
  updated_at?: number;
}

interface RawRobotModuleStatusPayload {
  power_manager?: number;
  rfid?: number;
  light_strip?: number;
  small_shooter?: number;
  big_shooter?: number;
  uwb?: number;
  armor?: number;
  video_transmission?: number;
  capacitor?: number;
  main_controller?: number;
  laser_detection_module?: number;
  updated_at?: number;
}

interface RawRobotPositionPayload {
  x?: number;
  y?: number;
  z?: number;
  yaw?: number;
  updated_at?: number;
}

interface RawRadarInfoPayload {
  target_robot_id?: number;
  target_pos_x?: number;
  target_pos_y?: number;
  torward_angle?: number;
  is_high_light?: number;
  updated_at?: number;
}

interface RawBuffPayload {
  robot_id?: number;
  buff_type?: number;
  buff_level?: number;
  buff_max_time?: number;
  buff_left_time?: number;
  updated_at?: number;
}

interface RawDeployModeStatusPayload {
  status?: number;
  updated_at?: number;
}

const robotDataStore = useRobotDataStore();
const uiPanelStore = useUiPanelStore();

watch(
  () => uiPanelStore.applied.resolution,
  (res) => {
    const [w, h] = res.split("x");
    document.documentElement.style.setProperty("--design-width", w);
    document.documentElement.style.setProperty("--design-height", h);
  },
  { immediate: true },
);

const videoState = ref<RawVideoStatePayload>(createDefaultVideoState());
const runtimeBridgeError = ref(false);

const showBlockingVideoOverlay = computed(
  () => !runtimeBridgeError.value && videoState.value.display_state !== "live",
);

const pipSourceLabel = computed(() =>
  videoState.value.pip_source === "custom" ? "自定义" : "官方",
);

const pipDisplayState = computed(() =>
  videoState.value.pip_source === "custom"
    ? videoState.value.custom_display_state
    : videoState.value.official_display_state,
);

const pipVideoLive = computed(() =>
  videoState.value.pip_source === "custom"
    ? videoState.value.custom_video_connected
    : videoState.value.official_video_connected,
);

const pipLayerCanExposeNativeVideo = computed(
  () =>
    pipVideoLive.value ||
    ((pipDisplayState.value === "waiting_frame" ||
      pipDisplayState.value === "resyncing") &&
      (videoState.value.pip_source === "official" ||
        videoState.value.active_source === "custom")),
);

// What: 主源黑幕只遮住主画面，不能盖住正在恢复或已经出画的 PiP 原生窗口。
// Why: PiP 的 ffplay 窗在 WebView 下层，幕布必须按同一几何留洞，否则用户会看到“PiP 在线但黑屏”。
const pipShouldPunchThrough = computed(
  () => showBlockingVideoOverlay.value && pipLayerCanExposeNativeVideo.value,
);

const pipMessage = computed(() =>
  videoState.value.pip_source === "custom"
    ? videoState.value.custom_message
    : videoState.value.official_message,
);

const pipStateLabel = computed(() => {
  if (pipDisplayState.value === "live") return "在线";
  if (pipDisplayState.value === "waiting_source") return "等待源";
  if (pipDisplayState.value === "waiting_frame") return "等待首帧";
  if (pipDisplayState.value === "resyncing") return "重同步";
  if (pipDisplayState.value === "stalled") return "已断开";
  return "异常";
});

const pipStatusTitle = computed(() => {
  if (videoState.value.pip_source === "custom") {
    if (pipDisplayState.value === "waiting_source") return "等待自定义 PiP";
    if (pipDisplayState.value === "waiting_frame") return "自定义 PiP 待出画";
    if (pipDisplayState.value === "stalled") return "自定义 PiP 已断开";
    if (pipDisplayState.value === "runtime_error") return "自定义 PiP 异常";
    return "自定义 PiP";
  }

  if (pipDisplayState.value === "waiting_source") return "等待官方 PiP";
  if (pipDisplayState.value === "waiting_frame") return "官方 PiP 待出画";
  if (pipDisplayState.value === "stalled") return "官方 PiP 已断开";
  if (pipDisplayState.value === "runtime_error") return "官方 PiP 异常";
  return "官方 PiP";
});

// What: 幕布标题严格跟随当前激活源。
// Why: 现在同时存在 official/custom 两类入口，若继续只写“官方视频断开”，用户会误判到底是切源失败还是官方源掉线。
const videoStatusTitle = computed(() => {
  if (videoState.value.active_source === "custom") {
    if (videoState.value.display_state === "waiting_source")
      return "等待自定义视频";
    if (videoState.value.display_state === "waiting_frame")
      return "自定义视频等待关键帧";
    if (videoState.value.display_state === "resyncing")
      return "自定义视频重同步中";
    if (videoState.value.display_state === "stalled") return "自定义视频已断开";
    if (videoState.value.display_state === "runtime_error")
      return "自定义视频源异常";
    return "自定义视频未出画";
  }

  if (videoState.value.display_state === "waiting_source") return "等待官方视频";
  if (videoState.value.display_state === "waiting_frame")
    return "等待完整视频帧";
  if (videoState.value.display_state === "resyncing") return "视频重同步中";
  if (videoState.value.display_state === "stalled") return "官方视频已断开";
  if (videoState.value.display_state === "runtime_error")
    return "原生视频层异常";
  return "官方视频正常";
});

// What: 幕布副信息统一展示当前源、备用源和实时指标。
// Why: 极简版本去掉了复杂诊断页，用户需要直接在主幕布上看到最关键的源切换与活跃信息。
const videoStatusMeta = computed(() => {
  if (runtimeBridgeError.value || videoState.value.display_state === "live")
    return "";

  const parts: string[] = [];
  parts.push(
    videoState.value.active_source === "custom" ? "当前源 自定义" : "当前源 官方",
  );
  parts.push(
    videoState.value.pip_source === "custom" ? "PiP 自定义" : "PiP 官方",
  );

  if (videoState.value.active_source !== "official") {
    parts.push(
      videoState.value.official_available ? "官方源在线" : "官方源离线",
    );
  }

  if (videoState.value.active_source !== "custom") {
    parts.push(
      videoState.value.custom_available ? "自定义源在线" : "自定义源离线",
    );
  }

  const sourceLatency =
    videoState.value.active_source === "custom"
      ? videoState.value.custom_latency_ms
      : videoState.value.official_latency_ms;
  if (sourceLatency > 0) {
    parts.push(`当前延迟 ${sourceLatency}ms`);
  }

  if (
    videoState.value.active_source === "custom" &&
    videoState.value.custom_h264_nal_total > 0
  ) {
    parts.push(
      `H264 sps=${videoState.value.custom_h264_sps} pps=${videoState.value.custom_h264_pps} idr=${videoState.value.custom_h264_idr}`,
    );
  }

  if (videoState.value.present_fps > 0) {
    parts.push(`显示 ${videoState.value.present_fps.toFixed(1)}fps`);
  }

  return parts.join(" · ");
});

let heartbeatTimer: number | undefined;
let mountTs = 0;
let lastVideoStateAt = 0;
const runtimeEventDisposers: Array<() => void> = [];

type DebugHudBridge = {
  setCombat: (patch: Partial<CombatData>) => void;
  setMatch: (patch: Partial<MatchState>) => void;
  setRadar: (patch: Partial<RadarData>) => void;
  setVideoState: (patch: Partial<RawVideoStatePayload>) => void;
  setVision: (patch: Partial<VisionData>) => void;
  reset: () => void;
};

// What: 构造一份稳定默认 video-state。
// Why: 后端首个事件到来前，幕布和诊断卡都必须拥有一份完整结构，不能靠模板判空。
function createDefaultVideoState(): RawVideoStatePayload {
  return {
    backend_connected: false,
    control_link_connected: false,
    video_connected: false,
    active_source: "official",
    pip_source: "custom",
    official_available: false,
    custom_available: false,
    official_video_connected: false,
    custom_video_connected: false,
    display_state: "waiting_source",
    official_display_state: "waiting_source",
    custom_display_state: "waiting_source",
    decoder_fps: 0,
    present_fps: 0,
    latency_ms: 0,
    official_latency_ms: 0,
    custom_latency_ms: 0,
    official_packet_rate_hz: 0,
    official_frame_rate_hz: 0,
    official_drop_rate: 0,
    custom_block_rate_hz: 0,
    custom_au_fps: 0,
    custom_drop_rate: 0,
    custom_corrupt_rate: 0,
    custom_h264_nal_total: 0,
    custom_h264_idr: 0,
    custom_h264_sps: 0,
    custom_h264_pps: 0,
    custom_h264_no_start_code: 0,
    custom_h264_synced: false,
    udp_drop_frames: 0,
    decode_drop_frames: 0,
    corrupt_frames: 0,
    decoder_resets: 0,
    header_order: "unknown",
    message: "等待视频状态",
    official_message: "等待官方视频状态",
    custom_message: "等待自定义视频状态",
    updated_at: 0,
  };
}

function normalizeVideoSource(value: unknown): VideoSource {
  return value === "custom" ? "custom" : "official";
}

function normalizeVideoDisplayState(value: unknown): VideoDisplayState {
  if (
    value === "live" ||
    value === "waiting_source" ||
    value === "waiting_frame" ||
    value === "resyncing" ||
    value === "stalled" ||
    value === "runtime_error"
  ) {
    return value;
  }
  return "waiting_source";
}

// What: 将后端 video-state 做字段级归一化。
// Why: 前端主幕布和诊断卡都直接依赖这份结构，必须先把异常值挡掉，避免源切换时出现 undefined 文案。
function normalizeVideoState(raw: Partial<RawVideoStatePayload>): RawVideoStatePayload {
  return {
    backend_connected: Boolean(raw.backend_connected ?? false),
    control_link_connected: Boolean(raw.control_link_connected ?? false),
    video_connected: Boolean(raw.video_connected ?? false),
    active_source: normalizeVideoSource(raw.active_source),
    pip_source:
      normalizeVideoSource(raw.pip_source) === normalizeVideoSource(raw.active_source)
        ? normalizeVideoSource(raw.active_source) === "custom"
          ? "official"
          : "custom"
        : normalizeVideoSource(raw.pip_source),
    official_available: Boolean(raw.official_available ?? false),
    custom_available: Boolean(raw.custom_available ?? false),
    official_video_connected: Boolean(raw.official_video_connected ?? false),
    custom_video_connected: Boolean(raw.custom_video_connected ?? false),
    display_state: normalizeVideoDisplayState(raw.display_state),
    official_display_state: normalizeVideoDisplayState(
      raw.official_display_state,
    ),
    custom_display_state: normalizeVideoDisplayState(raw.custom_display_state),
    decoder_fps: Math.max(0, Number(raw.decoder_fps ?? 0)),
    present_fps: Math.max(0, Number(raw.present_fps ?? 0)),
    latency_ms: Math.max(0, Math.round(Number(raw.latency_ms ?? 0))),
    official_latency_ms: Math.max(
      0,
      Math.round(Number(raw.official_latency_ms ?? raw.latency_ms ?? 0)),
    ),
    custom_latency_ms: Math.max(
      0,
      Math.round(Number(raw.custom_latency_ms ?? 0)),
    ),
    official_packet_rate_hz: Math.max(
      0,
      Number(raw.official_packet_rate_hz ?? 0),
    ),
    official_frame_rate_hz: Math.max(
      0,
      Number(raw.official_frame_rate_hz ?? 0),
    ),
    official_drop_rate: Math.min(
      1,
      Math.max(0, Number(raw.official_drop_rate ?? 0)),
    ),
    custom_block_rate_hz: Math.max(
      0,
      Number(raw.custom_block_rate_hz ?? 0),
    ),
    custom_au_fps: Math.max(0, Number(raw.custom_au_fps ?? 0)),
    custom_drop_rate: Math.min(
      1,
      Math.max(0, Number(raw.custom_drop_rate ?? 0)),
    ),
    custom_corrupt_rate: Math.min(
      1,
      Math.max(0, Number(raw.custom_corrupt_rate ?? 0)),
    ),
    custom_h264_nal_total: Math.max(
      0,
      Math.round(Number(raw.custom_h264_nal_total ?? 0)),
    ),
    custom_h264_idr: Math.max(
      0,
      Math.round(Number(raw.custom_h264_idr ?? 0)),
    ),
    custom_h264_sps: Math.max(
      0,
      Math.round(Number(raw.custom_h264_sps ?? 0)),
    ),
    custom_h264_pps: Math.max(
      0,
      Math.round(Number(raw.custom_h264_pps ?? 0)),
    ),
    custom_h264_no_start_code: Math.max(
      0,
      Math.round(Number(raw.custom_h264_no_start_code ?? 0)),
    ),
    custom_h264_synced: Boolean(raw.custom_h264_synced ?? false),
    udp_drop_frames: Math.max(0, Math.round(Number(raw.udp_drop_frames ?? 0))),
    decode_drop_frames: Math.max(
      0,
      Math.round(Number(raw.decode_drop_frames ?? 0)),
    ),
    corrupt_frames: Math.max(0, Math.round(Number(raw.corrupt_frames ?? 0))),
    decoder_resets: Math.max(0, Math.round(Number(raw.decoder_resets ?? 0))),
    header_order: String(raw.header_order ?? "unknown"),
    message: String(raw.message ?? "等待视频状态"),
    official_message: String(raw.official_message ?? "等待官方视频状态"),
    custom_message: String(raw.custom_message ?? "等待自定义视频状态"),
    updated_at: Number(raw.updated_at ?? Date.now()),
  };
}

// What: 将当前源健康度映射为统一链路质量。
// Why: 诊断卡已经缩成一个小卡片，必须继续复用单一 linkQuality 语义，不能让每个组件重新猜测 current source 的好坏。
function resolveLinkQuality(state: RawVideoStatePayload): "excellent" | "good" | "fair" | "poor" | "offline" {
  if (!state.backend_connected) return "offline";
  if (state.active_source === "custom") return state.custom_available ? "fair" : "poor";
  if (state.latency_ms <= 50) return "excellent";
  if (state.latency_ms <= 120) return "good";
  if (state.latency_ms <= 250) return "fair";
  return "poor";
}

// What: 将归一化后的 video-state 写入统一连接态 store。
// Why: HUD 各卡片不能直接依赖原始事件对象，必须继续通过 store 共享同一份源状态。
function applyVideoStateToStore(state: RawVideoStatePayload): void {
  robotDataStore.setConnectionState({
    backendConnected: state.backend_connected,
    videoConnected: state.video_connected,
    controlLinkConnected: state.control_link_connected,
    activeSource: state.active_source,
    pipSource: state.pip_source,
    officialAvailable: state.official_available,
    customAvailable: state.custom_available,
    officialVideoConnected: state.official_video_connected,
    customVideoConnected: state.custom_video_connected,
    videoDisplayState: state.display_state,
    officialDisplayState: state.official_display_state,
    customDisplayState: state.custom_display_state,
    stale: state.display_state !== "live",
    latencyMs: state.latency_ms,
    officialLatencyMs: state.official_latency_ms,
    customLatencyMs: state.custom_latency_ms,
    officialPacketRateHz: state.official_packet_rate_hz,
    officialFrameRateHz: state.official_frame_rate_hz,
    officialDropRate: state.official_drop_rate,
    customBlockRateHz: state.custom_block_rate_hz,
    customAUFPS: state.custom_au_fps,
    customDropRate: state.custom_drop_rate,
    customCorruptRate: state.custom_corrupt_rate,
    linkQuality: resolveLinkQuality(state),
    lastHeartbeatAt: state.updated_at,
    udpDropFrames: state.udp_drop_frames,
    decodeDropFrames: state.decode_drop_frames,
    corruptFrames: state.corrupt_frames,
    decoderResets: state.decoder_resets,
    headerOrder: state.header_order,
    message: state.message,
    officialMessage: state.official_message,
    customMessage: state.custom_message,
  });
}

function normalizeRole(value: unknown): TeamRole {
  if (
    value === "hero" ||
    value === "engineer" ||
    value === "infantry" ||
    value === "sentry"
  ) {
    return value;
  }
  return "unknown";
}

function resolveRoleLabel(role: TeamRole): string {
  if (role === "hero") return "英雄";
  if (role === "engineer") return "工程";
  if (role === "infantry") return "步兵";
  if (role === "sentry") return "哨兵";
  return "未知";
}

// What: 将单个队伍槽位转成前端稳定结构。
// Why: 顶部队伍血量卡严格依赖 `robotId / role / hp / maxHp` 这些字段，不能直接吃后端原始 snake_case。
function normalizeMatchUnit(raw: RawMatchUnitPayload): TeamRobotState {
  const role = normalizeRole(raw.role);
  return {
    robotId: Math.max(0, Math.round(Number(raw.robot_id ?? 0))),
    role,
    roleLabel: resolveRoleLabel(role),
    hp: Math.max(0, Number(raw.hp ?? 0)),
    maxHp: Math.max(1, Number(raw.max_hp ?? 1)),
    avatarKey: String(raw.avatar_key ?? ""),
    online: Boolean(raw.online ?? false),
  };
}

function normalizeMatchSide(raw: RawMatchSidePayload | undefined): MatchState["red"] {
  return {
    baseHp: Math.max(0, Number(raw?.base_hp ?? 0)),
    baseMaxHp: Math.max(1, Number(raw?.base_max_hp ?? 1)),
    baseStatus: Math.max(0, Math.round(Number(raw?.base_status ?? 0))),
    baseShield: Math.max(0, Math.round(Number(raw?.base_shield ?? 0))),
    outpostHp: Math.max(0, Number(raw?.outpost_hp ?? 0)),
    outpostStatus: Math.max(0, Math.round(Number(raw?.outpost_status ?? 0))),
    totalDamage: Math.max(0, Math.round(Number(raw?.total_damage ?? 0))),
    units: Array.isArray(raw?.units)
      ? raw.units.map((item) => normalizeMatchUnit(item ?? {}))
      : [],
  };
}

// What: 比赛态只保留顶部 HUD 真正在用的字段。
// Why: 本轮目标是围绕视频主视图收缩功能，因此这里不再桥接任何雷达、经济趋势或衍生提醒。
function normalizeMatchState(raw: Partial<RawMatchStatePayload>): Partial<MatchState> {
  const patch: Partial<MatchState> = {
    updatedAt: Number(raw.updated_at ?? Date.now()),
    origin: "backend",
  };

  // What: 只在后端显式带出某一块比赛态时才覆盖那一块。
  // Why: match-state 会由多个 topic 分批拼装，若这里把缺失字段直接归零，就会把另一条链已经到达的真实值洗掉。
  if (raw.red) patch.red = normalizeMatchSide(raw.red);
  if (raw.blue) patch.blue = normalizeMatchSide(raw.blue);
  if (raw.global_status_ready !== undefined) {
    patch.globalStatusReady = Boolean(raw.global_status_ready);
  }
  if (raw.game_status_ready !== undefined) {
    patch.gameStatusReady = Boolean(raw.game_status_ready);
  }
  if (raw.current_round !== undefined) {
    patch.currentRound = Math.max(0, Math.round(Number(raw.current_round)));
  }
  if (raw.total_rounds !== undefined) {
    patch.totalRounds = Math.max(0, Math.round(Number(raw.total_rounds)));
  }
  if (raw.red_score !== undefined) {
    patch.redScore = Math.max(0, Math.round(Number(raw.red_score)));
  }
  if (raw.blue_score !== undefined) {
    patch.blueScore = Math.max(0, Math.round(Number(raw.blue_score)));
  }
  if (raw.current_stage !== undefined) {
    patch.currentStage = Math.max(0, Math.round(Number(raw.current_stage)));
  }
  if (raw.stage_countdown_sec !== undefined) {
    patch.stageCountdownSec = Math.max(
      0,
      Math.round(Number(raw.stage_countdown_sec)),
    );
  }
  if (raw.stage_elapsed_sec !== undefined) {
    patch.stageElapsedSec = Math.max(
      0,
      Math.round(Number(raw.stage_elapsed_sec)),
    );
  }
  if (raw.is_paused !== undefined) {
    patch.isPaused = Boolean(raw.is_paused);
  }

  return patch;
}

// What: 战斗态只保留本车状态卡和火控卡需要的字段。
// Why: 输入控制和快捷动作已经被砍掉，不必再为这些功能桥接额外派生状态。
function normalizeCombatState(raw: Partial<RawCombatStatePayload>): Partial<CombatData> {
  const patch: Partial<CombatData> = {
    updatedAt: Number(raw.updated_at ?? Date.now()),
    origin: "backend",
  };

  // What: 只覆盖后端本次确实给出的战斗态字段。
  // Why: combat-state 目前仍是由多条官方状态拼装而来，缺失字段必须保留前一份真实值，不能被默认值冲掉。
  if (raw.hp !== undefined) patch.hp = Math.max(0, Number(raw.hp));
  if (raw.max_hp !== undefined) patch.maxHp = Math.max(1, Number(raw.max_hp));
  if (raw.ammo !== undefined) patch.ammo = Math.max(0, Number(raw.ammo));
  if (raw.max_ammo !== undefined) {
    patch.maxAmmo = Math.max(1, Number(raw.max_ammo));
  }
  if (raw.heat !== undefined) patch.heat = Math.max(0, Number(raw.heat));
  if (raw.max_heat !== undefined) {
    patch.maxHeat = Math.max(1, Number(raw.max_heat));
  }
  if (raw.last_projectile_fire_rate !== undefined) {
    patch.lastProjectileFireRate = Math.max(0, Number(raw.last_projectile_fire_rate));
  }
  if (raw.current_chassis_energy !== undefined) {
    patch.chassisEnergy = Math.max(0, Number(raw.current_chassis_energy));
  }
  if (raw.current_buffer_energy !== undefined) {
    patch.bufferEnergy = Math.max(0, Number(raw.current_buffer_energy));
  }
  if (raw.current_experience !== undefined) {
    patch.currentExperience = Math.max(0, Number(raw.current_experience));
  }
  if (raw.experience_for_upgrade !== undefined) {
    patch.experienceForUpgrade = Math.max(0, Number(raw.experience_for_upgrade));
  }
  if (raw.total_projectiles_fired !== undefined) {
    patch.totalProjectilesFired = Math.max(0, Math.round(Number(raw.total_projectiles_fired)));
  }
  if (raw.is_out_of_combat !== undefined) {
    patch.isOutOfCombat = Boolean(raw.is_out_of_combat);
  }
  if (raw.out_of_combat_countdown_sec !== undefined) {
    patch.outOfCombatCountdownSec = Math.max(0, Math.round(Number(raw.out_of_combat_countdown_sec)));
  }
  if (raw.can_remote_heal !== undefined) {
    patch.canRemoteHeal = Boolean(raw.can_remote_heal);
  }
  if (raw.bullet_speed !== undefined) {
    patch.bulletSpeed = Math.max(0, Number(raw.bullet_speed));
  }
  if (raw.capacitor_pct !== undefined) {
    patch.capacitorPct = Math.max(
      0,
      Math.min(100, Number(raw.capacitor_pct)),
    );
  }
  if (raw.can_remote_ammo !== undefined) {
    patch.canRemoteAmmo = Boolean(raw.can_remote_ammo);
  }

  return patch;
}

function normalizeGlobalLogistics(raw: Partial<RawGlobalLogisticsPayload>): Partial<GlobalLogisticsState> {
  return {
    remainingEconomy: Math.max(0, Math.round(Number(raw.remaining_economy ?? 0))),
    totalEconomyObtained: Math.max(0, Math.round(Number(raw.total_economy_obtained ?? 0))),
    techLevel: Math.max(0, Math.round(Number(raw.tech_level ?? 0))),
    encryptionLevel: Math.max(0, Math.round(Number(raw.encryption_level ?? 0))),
    updatedAt: Number(raw.updated_at ?? Date.now()),
  };
}

function normalizeSpecialMechanism(raw: Partial<RawSpecialMechanismPayload>): Partial<GlobalSpecialMechanismState> {
  return {
    mechanisms: Array.isArray(raw.mechanisms)
      ? raw.mechanisms.map((item) => ({
          mechanismId: Math.max(0, Math.round(Number(item?.mechanism_id ?? 0))),
          timeSec: Math.max(0, Math.round(Number(item?.time_sec ?? 0))),
        }))
      : [],
    updatedAt: Number(raw.updated_at ?? Date.now()),
  };
}

function normalizeRefereeEvent(raw: Partial<RawRefereeEventPayload>): RefereeEventEntry {
  const updatedAt = Number(raw.updated_at ?? Date.now());
  const eventId = Math.max(0, Math.round(Number(raw.event_id ?? 0)));
  const param = String(raw.param ?? "");
  return {
    id: `${updatedAt}:${eventId}:${param}`,
    eventId,
    param,
    updatedAt,
  };
}

function normalizeRobotStaticStatus(raw: Partial<RawRobotStaticStatusPayload>): Partial<RobotStaticStatusState> {
  return {
    connectionState: Math.max(0, Math.round(Number(raw.connection_state ?? 0))),
    fieldState: Math.max(0, Math.round(Number(raw.field_state ?? 0))),
    aliveState: Math.max(0, Math.round(Number(raw.alive_state ?? 0))),
    robotId: Math.max(0, Math.round(Number(raw.robot_id ?? 0))),
    robotType: Math.max(0, Math.round(Number(raw.robot_type ?? 0))),
    performanceSystemShooter: Math.max(0, Math.round(Number(raw.performance_system_shooter ?? 0))),
    performanceSystemChassis: Math.max(0, Math.round(Number(raw.performance_system_chassis ?? 0))),
    level: Math.max(0, Math.round(Number(raw.level ?? 0))),
    maxHealth: Math.max(0, Math.round(Number(raw.max_health ?? 0))),
    maxHeat: Math.max(0, Number(raw.max_heat ?? 0)),
    heatCooldownRate: Math.max(0, Number(raw.heat_cooldown_rate ?? 0)),
    maxPower: Math.max(0, Math.round(Number(raw.max_power ?? 0))),
    maxBufferEnergy: Math.max(0, Math.round(Number(raw.max_buffer_energy ?? 0))),
    maxChassisEnergy: Math.max(0, Math.round(Number(raw.max_chassis_energy ?? 0))),
    updatedAt: Number(raw.updated_at ?? Date.now()),
  };
}

function normalizeRobotModuleStatus(raw: Partial<RawRobotModuleStatusPayload>): Partial<RobotModuleStatusState> {
  return {
    powerManager: Math.max(0, Math.round(Number(raw.power_manager ?? 0))),
    rfid: Math.max(0, Math.round(Number(raw.rfid ?? 0))),
    lightStrip: Math.max(0, Math.round(Number(raw.light_strip ?? 0))),
    smallShooter: Math.max(0, Math.round(Number(raw.small_shooter ?? 0))),
    bigShooter: Math.max(0, Math.round(Number(raw.big_shooter ?? 0))),
    uwb: Math.max(0, Math.round(Number(raw.uwb ?? 0))),
    armor: Math.max(0, Math.round(Number(raw.armor ?? 0))),
    videoTransmission: Math.max(0, Math.round(Number(raw.video_transmission ?? 0))),
    capacitor: Math.max(0, Math.round(Number(raw.capacitor ?? 0))),
    mainController: Math.max(0, Math.round(Number(raw.main_controller ?? 0))),
    laserDetectionModule: Math.max(0, Math.round(Number(raw.laser_detection_module ?? 0))),
    updatedAt: Number(raw.updated_at ?? Date.now()),
  };
}

function normalizeRobotPosition(raw: Partial<RawRobotPositionPayload>) {
  return {
    positionX: Number(raw.x ?? 0),
    positionY: Number(raw.y ?? 0),
    positionZ: Number(raw.z ?? 0),
    headingDeg: Number(raw.yaw ?? 0),
    updatedAt: Number(raw.updated_at ?? Date.now()),
    origin: "backend" as const,
  };
}

function normalizeRadarInfo(raw: Partial<RawRadarInfoPayload>): Partial<RadarData> {
  const updatedAt = Number(raw.updated_at ?? Date.now());
  return {
    source: "backend",
    updatedAt,
    contacts: [
      {
        id: String(raw.target_robot_id ?? ""),
        x: Number(raw.target_pos_x ?? 0),
        y: Number(raw.target_pos_y ?? 0),
        velocity: 0,
        side: "enemy",
        confidence: 1,
        headingDeg: Number(raw.torward_angle ?? 0),
        highlighted: Number(raw.is_high_light ?? 0) > 0,
        updatedAt,
      },
    ],
  };
}

function normalizeBuff(raw: Partial<RawBuffPayload>): BuffState {
  return {
    robotId: Math.max(0, Math.round(Number(raw.robot_id ?? 0))),
    buffType: Math.max(0, Math.round(Number(raw.buff_type ?? 0))),
    buffLevel: Math.round(Number(raw.buff_level ?? 0)),
    buffMaxTime: Math.max(0, Math.round(Number(raw.buff_max_time ?? 0))),
    buffLeftTime: Math.max(0, Math.round(Number(raw.buff_left_time ?? 0))),
    updatedAt: Number(raw.updated_at ?? Date.now()),
  };
}

function normalizeDeployModeStatus(raw: Partial<RawDeployModeStatusPayload>): Partial<DeployModeStatusState> {
  return {
    status: Math.max(0, Math.round(Number(raw.status ?? 0))),
    updatedAt: Number(raw.updated_at ?? Date.now()),
  };
}

function bindRuntimeEvent(
  eventName: string,
  handler: (payload: Record<string, unknown>) => void,
): void {
  const runtimeApi = (window as unknown as {
    runtime?: { EventsOn?: (name: string, fn: (payload: Record<string, unknown>) => void) => (() => void) | void }
  }).runtime

  if (!runtimeApi?.EventsOn) return

  const disposer = runtimeApi.EventsOn(eventName, handler)
  if (typeof disposer === "function") runtimeEventDisposers.push(disposer)
}

// What: 在开发态暴露一组最小调试桥给 Playwright 与本地联调使用。
// Why: HUD 的大部分回归用例依赖直接注入 store 状态；若没有这层桥，测试环境就只能卡死在“未检测到运行时事件桥”。
function installDebugHudBridge(): void {
  if (!import.meta.env.DEV || typeof window === "undefined") return

  const debugWindow = window as Window & { __RM_DEBUG_HUD__?: DebugHudBridge }

  // What: 所有调试入口先统一清掉 runtime bridge 错误态。
  // Why: Playwright 跑的是纯前端 Vite 环境，没有 Wails runtime；若不先把这个错误态放下，HUD 永远只会显示致命错误遮罩。
  const activateDebugSession = (): void => {
    runtimeBridgeError.value = false
  }

  debugWindow.__RM_DEBUG_HUD__ = {
    // What: 直接把战斗态 patch 注入统一 store。
    // Why: 这样测试可以复用真实组件消费路径，而不是再维护一套专用 mock 组件。
    setCombat(patch) {
      activateDebugSession()
      robotDataStore.applyCombatPatch(patch)
    },

    // What: 直接把比赛态 patch 注入统一 store。
    // Why: 顶部比分条、基地血条和编组卡都共享这份状态，调试桥必须走同一入口才能保证测试可信。
    setMatch(patch) {
      activateDebugSession()
      robotDataStore.applyMatchStatePatch(patch)
    },

    // What: 直接把雷达态 patch 注入统一 store。
    // Why: 态势增强器的回归用例需要独立控制雷达输入，不能依赖真实后端事件时序。
    setRadar(patch) {
      activateDebugSession()
      robotDataStore.applyRadarPatch(patch)
    },

    // What: 先复用现有 video-state 归一化，再同步写入 video ref 与连接态 store。
    // Why: 视频幕布、状态卡和链路诊断都依赖同一份 video-state 语义，调试桥不能绕开这条归一化路径。
    setVideoState(patch) {
      activateDebugSession()
      const nextState = normalizeVideoState(patch)
      lastVideoStateAt = performance.now()
      videoState.value = nextState
      applyVideoStateToStore(nextState)
    },

    // What: 直接把视觉识别态 patch 注入统一 store。
    // Why: 态势增强器的目标提示与准星强化都依赖视觉态，调试桥必须允许单独构造这类输入。
    setVision(patch) {
      activateDebugSession()
      robotDataStore.applyVisionPatch(patch)
    },

    // What: 同时重置前端 video ref 与统一 telemetry store。
    // Why: 每条 E2E 用例都需要从干净状态开始，不能让上一条测试残留的 HUD 状态污染下一条断言。
    reset() {
      activateDebugSession()
      videoState.value = createDefaultVideoState()
      lastVideoStateAt = 0
      robotDataStore.resetToDefault()
    },
  }
}

// What: 定时检查后端视频状态心跳。
// Why: 当 Wails 事件桥还在但后端状态广播卡住时，主界面也必须在数秒内明确落入“当前源断开”而不是继续停在旧画面。
function startHeartbeatWatch(): void {
  heartbeatTimer = window.setInterval(() => {
    if (runtimeBridgeError.value) return

    const now = performance.now()
    const ageMs = lastVideoStateAt === 0 ? now - mountTs : now - lastVideoStateAt
    if (ageMs < 2500) return

    const stalledState = normalizeVideoState({
      ...videoState.value,
      backend_connected: false,
      video_connected: false,
      pip_source: videoState.value.pip_source,
      official_available: false,
      custom_available: false,
      official_video_connected: false,
      custom_video_connected: false,
      display_state: "stalled",
      official_display_state: "stalled",
      custom_display_state: "stalled",
      latency_ms: Math.round(ageMs),
      message:
        videoState.value.active_source === "custom"
          ? "自定义视频状态断开"
          : "官方视频状态断开",
      official_message: "官方视频状态断开",
      custom_message: "自定义视频状态断开",
      updated_at: Date.now(),
    })

    videoState.value = stalledState
    robotDataStore.markDisconnected(stalledState.message)
    robotDataStore.setConnectionState({
      activeSource: stalledState.active_source,
      pipSource: stalledState.pip_source,
      officialAvailable: stalledState.official_available,
      customAvailable: stalledState.custom_available,
      officialVideoConnected: stalledState.official_video_connected,
      customVideoConnected: stalledState.custom_video_connected,
      message: stalledState.message,
      officialMessage: stalledState.official_message,
      customMessage: stalledState.custom_message,
    })
  }, 500)
}

onMounted(() => {
  mountTs = performance.now()
  void uiPanelStore.hydrate()
  startHeartbeatWatch()
  installDebugHudBridge()

  const runtimeApi = (window as unknown as {
    runtime?: { EventsOn?: unknown }
  }).runtime

  if (!runtimeApi?.EventsOn) {
    runtimeBridgeError.value = true
    videoState.value = normalizeVideoState({
      ...createDefaultVideoState(),
      message: "未检测到运行时事件桥",
      updated_at: Date.now(),
    })
    robotDataStore.markDisconnected("未检测到运行时事件桥")
    return
  }

  bindRuntimeEvent("video-state", (payload) => {
    const nextState = normalizeVideoState(payload as Partial<RawVideoStatePayload>)
    runtimeBridgeError.value = false
    lastVideoStateAt = performance.now()
    videoState.value = nextState
    applyVideoStateToStore(nextState)
  })

  bindRuntimeEvent("match-state", (payload) => {
    robotDataStore.applyMatchStatePatch(
      normalizeMatchState(payload as Partial<RawMatchStatePayload>),
    )
  })

  bindRuntimeEvent("combat-state", (payload) => {
    robotDataStore.applyCombatPatch(
      normalizeCombatState(payload as Partial<RawCombatStatePayload>),
    )
  })

  bindRuntimeEvent("global-logistics-status", (payload) => {
    robotDataStore.applyGlobalLogisticsPatch(
      normalizeGlobalLogistics(payload as Partial<RawGlobalLogisticsPayload>),
    )
  })

  bindRuntimeEvent("global-special-mechanism", (payload) => {
    robotDataStore.applyGlobalSpecialMechanismPatch(
      normalizeSpecialMechanism(payload as Partial<RawSpecialMechanismPayload>),
    )
  })

  bindRuntimeEvent("referee-event", (payload) => {
    robotDataStore.pushRefereeEvent(
      normalizeRefereeEvent(payload as Partial<RawRefereeEventPayload>),
    )
  })

  bindRuntimeEvent("robot-static-status", (payload) => {
    robotDataStore.applyRobotStaticStatusPatch(
      normalizeRobotStaticStatus(payload as Partial<RawRobotStaticStatusPayload>),
    )
  })

  bindRuntimeEvent("robot-module-status", (payload) => {
    robotDataStore.applyRobotModuleStatusPatch(
      normalizeRobotModuleStatus(payload as Partial<RawRobotModuleStatusPayload>),
    )
  })

  bindRuntimeEvent("robot-position", (payload) => {
    robotDataStore.applyChassisPatch(
      normalizeRobotPosition(payload as Partial<RawRobotPositionPayload>),
    )
  })

  bindRuntimeEvent("radar-info", (payload) => {
    robotDataStore.applyRadarPatch(
      normalizeRadarInfo(payload as Partial<RawRadarInfoPayload>),
    )
  })

  bindRuntimeEvent("buff-state", (payload) => {
    robotDataStore.upsertBuff(
      normalizeBuff(payload as Partial<RawBuffPayload>),
    )
  })

  bindRuntimeEvent("deploy-mode-status", (payload) => {
    robotDataStore.applyDeployModeStatusPatch(
      normalizeDeployModeStatus(payload as Partial<RawDeployModeStatusPayload>),
    )
  })
})

onUnmounted(() => {
  if (heartbeatTimer !== undefined) window.clearInterval(heartbeatTimer)
  runtimeEventDisposers.splice(0).forEach((dispose) => dispose())
  if (import.meta.env.DEV && typeof window !== "undefined") {
    delete (window as Window & { __RM_DEBUG_HUD__?: DebugHudBridge }).__RM_DEBUG_HUD__
  }
})
</script>

<style scoped>
.app-stage {
  position: fixed;
  inset: 0;
  overflow: hidden;
  background: transparent;
  --pip-gap: clamp(12px, 1.4vw, 24px);
  --pip-slot-w: clamp(320px, 29vw, 560px);
  --pip-slot-h: calc(var(--pip-slot-w) * 9 / 16);
  --pip-reserve-right: calc(var(--pip-gap) + var(--pip-slot-w));
}

.app-stage--pip-custom {
  --pip-slot-w: clamp(260px, 24vw, 420px);
  --pip-slot-h: var(--pip-slot-w);
}

.app-video-curtain {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: var(--z-curtain);
}

.app-video-curtain__piece {
  position: absolute;
  background: #000;
}

.app-video-curtain:not(.app-video-curtain--pip-cutout)
  .app-video-curtain__piece--top {
  inset: 0;
}

.app-video-curtain:not(.app-video-curtain--pip-cutout)
  .app-video-curtain__piece--left,
.app-video-curtain:not(.app-video-curtain--pip-cutout)
  .app-video-curtain__piece--right,
.app-video-curtain:not(.app-video-curtain--pip-cutout)
  .app-video-curtain__piece--bottom {
  display: none;
}

/* What: 主画面等待时，幕布按 PiP 几何切出透明窗口。
   Why: 原生 PiP 窗口在 WebView 下层，只有留洞才能真正看见官方/自定义备用画面。 */
.app-video-curtain--pip-cutout .app-video-curtain__piece--top {
  left: 0;
  right: 0;
  top: 0;
  height: var(--pip-gap);
}

.app-video-curtain--pip-cutout .app-video-curtain__piece--left {
  left: 0;
  top: var(--pip-gap);
  width: calc(100% - var(--pip-gap) - var(--pip-slot-w));
  height: var(--pip-slot-h);
}

.app-video-curtain--pip-cutout .app-video-curtain__piece--right {
  right: 0;
  top: var(--pip-gap);
  width: var(--pip-gap);
  height: var(--pip-slot-h);
}

.app-video-curtain--pip-cutout .app-video-curtain__piece--bottom {
  left: 0;
  right: 0;
  top: calc(var(--pip-gap) + var(--pip-slot-h));
  bottom: 0;
}

.app-pip-slot {
  position: absolute;
  right: var(--pip-gap);
  top: var(--pip-gap);
  z-index: 8;
  width: var(--pip-slot-w);
  height: var(--pip-slot-h);
  border-radius: 22px;
  border: 1px solid rgba(116, 151, 230, 0.34);
  box-shadow:
    0 18px 42px rgba(0, 0, 0, 0.28),
    inset 0 0 0 1px rgba(17, 28, 51, 0.14);
  overflow: hidden;
  pointer-events: none;
  display: grid;
  grid-template-rows: auto 1fr;
  backdrop-filter: blur(3px);
}

.app-pip-slot.is-live {
  background: linear-gradient(
    165deg,
    rgba(6, 10, 18, 0.08),
    rgba(7, 11, 19, 0.02)
  );
}

.app-pip-slot.is-pass-through {
  background: transparent;
  backdrop-filter: none;
}

.app-pip-slot.is-offline:not(.is-pass-through) {
  background: linear-gradient(165deg, rgba(0, 0, 0, 0.94), rgba(4, 6, 11, 0.98));
}

.app-pip-slot__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 14px 0;
}

.app-pip-slot__badge,
.app-pip-slot__state {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  background: rgba(8, 14, 27, 0.66);
  color: rgba(223, 236, 255, 0.94);
  font-size: 11px;
  letter-spacing: 0.08em;
  box-shadow: 0 8px 18px rgba(0, 0, 0, 0.18);
}

.app-pip-slot__state {
  color: rgba(177, 204, 244, 0.92);
}

.app-pip-slot__empty {
  align-self: stretch;
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 6px;
  padding: 18px;
  text-align: center;
}

.app-pip-slot__title,
.app-pip-slot__sub {
  margin: 0;
}

.app-pip-slot__title {
  color: var(--hud-text-primary);
  font-size: 16px;
  letter-spacing: 0.08em;
}

.app-pip-slot__sub {
  color: var(--hud-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

/* What: 等待视频源提示沉到底图上层、HUD 下层。
   Why: 普通等待提示需要可见，但不能再遮住准星与实时 HUD。 */
.app-video-overlay {
  position: absolute;
  left: 50%;
  bottom: 28px;
  transform: translateX(-50%);
  z-index: var(--z-video-underlay);
  width: min(560px, calc(100vw - 48px));
  padding: 14px 18px;
  border-radius: 20px;
  border: 1px solid rgba(120, 147, 202, 0.2);
  background: linear-gradient(165deg, rgba(8, 12, 20, 0.78), rgba(5, 8, 14, 0.82));
  box-shadow: 0 16px 34px rgba(0, 0, 0, 0.28);
  display: grid;
  gap: 4px;
  text-align: center;
  pointer-events: none;
}

/* What: 运行时桥异常继续维持最高优先级。
   Why: 这是致命故障，必须盖住 HUD 并把错误直接抛给用户。 */
.app-runtime-status {
  position: absolute;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  z-index: var(--z-video-alert);
  width: min(560px, calc(100vw - 48px));
  padding: 22px 24px;
  border-radius: 22px;
  border: 1px solid rgba(120, 147, 202, 0.28);
  background: linear-gradient(165deg, rgba(8, 12, 20, 0.94), rgba(5, 8, 14, 0.96));
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.42);
  display: grid;
  gap: 8px;
  text-align: center;
  pointer-events: none;
}

.app-video-overlay--underlay .fallback-title {
  font-size: 18px;
}

.app-video-overlay--underlay .fallback-sub {
  font-size: 12px;
}

.app-video-overlay--underlay .fallback-meta {
  font-size: 11px;
}

.fallback-title {
  margin: 0;
  color: var(--hud-text-primary);
  font-size: 24px;
  letter-spacing: 0.08em;
}

.fallback-sub {
  margin: 0;
  color: var(--hud-text-secondary);
  font-size: 14px;
  line-height: 1.6;
}

.fallback-meta {
  margin: 0;
  color: rgba(196, 214, 245, 0.86);
  font-size: 12px;
  line-height: 1.6;
}

.app-scanlines {
  position: absolute;
  inset: 0;
  z-index: var(--z-scanlines);
  pointer-events: none;
  background-image: linear-gradient(
    to bottom,
    rgba(255, 255, 255, 0.015) 50%,
    rgba(0, 0, 0, 0.06) 50%
  );
  background-size: 100% 4px;
  mix-blend-mode: soft-light;
  opacity: 0.35;
}

@media (max-width: 980px) {
  .app-stage {
    --pip-gap: 12px;
    --pip-slot-w: min(46vw, 300px);
  }

  .app-stage--pip-custom {
    --pip-slot-w: min(38vw, 220px);
    --pip-slot-h: var(--pip-slot-w);
  }
}
</style>
