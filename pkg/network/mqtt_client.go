package network

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
	"google.golang.org/protobuf/proto"

	"rm-aurora/pkg/rmcp"
)

const (
	// What: MQTT 3.1.1 CONNECT 报文固定头字节。
	// Why: 这里的探测只需要完成最小合法握手，不需要引入额外 MQTT 库状态机。
	mqttPacketTypeConnect = 0x10

	// What: MQTT 3.1.1 CONNACK 报文固定头字节。
	// Why: 服务端若正确理解 CONNECT，就必须以这个报文类型回复连接结果。
	mqttPacketTypeConnack = 0x20

	// What: MQTT 3.1.1 协议版本号。
	// Why: 当前正式客户端与协议文档都按 3.1.1 工作，探测报文必须与正式连接保持一致。
	mqttProtocolLevel311 = 0x04

	// What: MQTT 3.1.1 成功连接返回码。
	// Why: 只有拿到 0x00，才能说明 broker 接受了当前 clientID。
	mqttConnackAccepted = 0x00

	// What: MQTT 3.1.1 标识符被拒绝返回码。
	// Why: 这正是现场当前碰到的故障类型，必须单独识别出来做 clientID 自动切换。
	mqttConnackIdentifierRejected = 0x02
)

type mqttSubscription struct {
	topic   string
	qos     byte
	handler pahomqtt.MessageHandler
}

type MQTTClient struct {
	client pahomqtt.Client

	mu            sync.Mutex
	broker        string
	subscriptions map[string]mqttSubscription
}

// IsConnected 返回当前 MQTT 连接是否可用。
// What: 为上层状态桥接提供最小只读探针。
// Why: HUD 连接态需要区分“视频到了但控制链断了”和“整机后端都断了”两类故障。
func (mc *MQTTClient) IsConnected() bool {
	return mc != nil && mc.client != nil && mc.client.IsConnected()
}

// NewMQTTClient 初始化裁判系统连接。
// What: 创建客户端时就挂上 OnConnect/OnConnectionLost 回调。
// Why: 初次连接慢、掉线重连或 broker 抖动时，订阅都必须能自动恢复，不能依赖上层手工重试。
func NewMQTTClient(serverIP string, port int, clientID uint16) (*MQTTClient, error) {
	broker := fmt.Sprintf("tcp://%s:%d", serverIP, port)

	mc := &MQTTClient{
		broker:        broker,
		subscriptions: make(map[string]mqttSubscription),
	}

	opts := pahomqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(fmt.Sprintf("%d", clientID)). // 裁判系统严格要求以客户端队伍分配 ID 标识
		SetProtocolVersion(4).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(3 * time.Second).
		SetMaxReconnectInterval(3 * time.Second)

	// What: 真正的“已连接”日志只在 OnConnect 里打。
	// Why: Connect token 的 WaitTimeout 只代表等待是否结束，不代表此刻一定已经建立连接。
	opts.SetOnConnectHandler(func(client pahomqtt.Client) {
		log.Printf("[network] MQTT connected to %s", broker)
		mc.resubscribeAll(client)
	})

	// What: 明确记录掉线原因。
	// Why: 比赛现场网络波动时，掉线日志是判断 broker/网口/布线问题的重要证据。
	opts.SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
		log.Printf("[Warning] MQTT connection lost: %v", err)
	})

	c := pahomqtt.NewClient(opts)
	mc.client = c

	connectToken := c.Connect()
	connectErr := pahomqtt.WaitTokenTimeout(connectToken, 5*time.Second)
	switch {
	case connectErr == nil:
		// What: 连接已在等待窗口内建立完成。
		// Why: 这种情况下 OnConnect 已经会负责补订阅，这里只需要返回实例即可。
		return mc, nil
	case errors.Is(connectErr, pahomqtt.TimedOut):
		// What: 初次连接 5 秒内尚未完成时，继续保留客户端实例。
		// Why: 现在已经启用 ConnectRetry，后续一旦连上会自动触发 OnConnect 并恢复订阅。
		log.Printf("[Warning] MQTT initial connect still pending after 5s, will keep retrying in background")
		return mc, nil
	default:
		return nil, connectErr
	}
}

// DetectAcceptedMQTTClientID 探测当前 broker 实际接受的机器人 clientID。
// What: 依次对候选 ID 发送最小合法 MQTT CONNECT，并读取 CONNACK 返回码。
// Why: 现场最常见问题是程序里写死了错误的机器人 ID，broker 会直接回 identifier rejected；这里把“猜 ID”改成自动探测。
func DetectAcceptedMQTTClientID(serverIP string, port int, preferredID uint16, candidates []uint16, timeout time.Duration) (uint16, error) {
	// What: 先把 preferredID 放到最前并对候选去重。
	// Why: 正确配置时应该尽量零额外成本通过；只有 preferredID 被拒绝时才继续横向探测其它机器人 ID。
	orderedCandidates := orderMQTTClientIDCandidates(preferredID, candidates)
	if len(orderedCandidates) == 0 {
		return 0, fmt.Errorf("no mqtt client id candidates configured")
	}

	var lastErr error

	for _, candidate := range orderedCandidates {
		// What: 对单个候选 ID 执行一次最小握手探测。
		// Why: 只要 broker 回 0x00，就能确认当前比赛环境真正允许哪个机器人身份接入。
		connackCode, err := probeMQTTClientID(serverIP, port, candidate, timeout)
		if err != nil {
			lastErr = err
			continue
		}

		if connackCode == mqttConnackAccepted {
			return candidate, nil
		}
	}

	if lastErr != nil {
		return 0, fmt.Errorf("mqtt client id probe failed: %w", lastErr)
	}

	return 0, fmt.Errorf("mqtt client id probe rejected all candidates: %v", orderedCandidates)
}

// orderMQTTClientIDCandidates 将首选 ID 与候选列表整理为稳定、去重的探测顺序。
// What: 把 preferredID 放在最前，随后拼接剩余候选。
// Why: 这样既能优先保留用户当前配置，又能在配置错误时自动向其它机器人 ID 回退。
func orderMQTTClientIDCandidates(preferredID uint16, candidates []uint16) []uint16 {
	// What: 用 map 去重。
	// Why: 候选表里可能同时包含 preferredID 与默认全集，若不去重会产生无意义重复探测。
	seen := make(map[uint16]struct{}, len(candidates)+1)
	ordered := make([]uint16, 0, len(candidates)+1)

	// What: 只要 preferredID 合法，就永远最先尝试。
	// Why: 用户当前配置本来就可能是正确的，不能让自动探测反过来先扰动其它 ID。
	if preferredID != 0 {
		seen[preferredID] = struct{}{}
		ordered = append(ordered, preferredID)
	}

	for _, candidate := range candidates {
		if candidate == 0 {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}

		seen[candidate] = struct{}{}
		ordered = append(ordered, candidate)
	}

	return ordered
}

// probeMQTTClientID 对单个候选 clientID 执行一次最小 MQTT 握手探测。
// What: 通过原始 TCP 连接发送 CONNECT，再读取 4 字节 CONNACK。
// Why: 这样可以在真正创建 Paho 客户端前，快速判断 broker 是否接受当前机器人身份。
func probeMQTTClientID(serverIP string, port int, clientID uint16, timeout time.Duration) (byte, error) {
	if serverIP == "" || port <= 0 {
		return 0, fmt.Errorf("invalid mqtt probe target %s:%d", serverIP, port)
	}
	if clientID == 0 {
		return 0, fmt.Errorf("invalid mqtt probe client id: %d", clientID)
	}
	if timeout <= 0 {
		timeout = time.Second
	}

	address := fmt.Sprintf("%s:%d", serverIP, port)

	// What: 使用超时受控的 TCP 连接。
	// Why: 探测只服务于启动期自动识别，绝不能因为对端卡死把主线程长时间拖住。
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// What: 整个握手过程统一挂同一个 deadline。
	// Why: 这样读写任意一步卡住都会按同一时限快速失败，不会留下半吊子 socket。
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return 0, err
	}

	packet, err := buildMQTTConnectPacket(clientID)
	if err != nil {
		return 0, err
	}

	// What: 先把最小合法 CONNECT 报文发给 broker。
	// Why: 只有拿到服务端 CONNACK，才能准确知道当前 clientID 是否被接受。
	if _, err := conn.Write(packet); err != nil {
		return 0, err
	}

	reply := make([]byte, 4)

	// What: MQTT 3.1.1 CONNACK 固定长度就是 4 字节。
	// Why: 探测逻辑只关心会话存在位与返回码，不需要解析更复杂的数据帧。
	if _, err := io.ReadFull(conn, reply); err != nil {
		return 0, err
	}

	// What: 严格校验返回帧头。
	// Why: 若这里不是标准 CONNACK，就说明对端并不是当前协议预期的 MQTT broker，继续推断 clientID 没有意义。
	if reply[0] != mqttPacketTypeConnack || reply[1] != 0x02 {
		return 0, fmt.Errorf("unexpected mqtt connack header=%x len=%d", reply[0], reply[1])
	}

	return reply[3], nil
}

// buildMQTTConnectPacket 构造最小合法的 MQTT 3.1.1 CONNECT 报文。
// What: 使用 clean session + 60 秒 keepalive，只携带 clientID。
// Why: 探测目标只是验证 broker 接不接受该机器人 ID，不需要用户名、密码或额外属性。
func buildMQTTConnectPacket(clientID uint16) ([]byte, error) {
	if clientID == 0 {
		return nil, fmt.Errorf("invalid mqtt connect client id: %d", clientID)
	}

	clientIDBytes := []byte(fmt.Sprintf("%d", clientID))

	// What: 可变头固定为 MQTT 3.1.1 + CleanSession + KeepAlive。
	// Why: 与正式客户端保持一致，才能保证探测结果和真实连接结果不会出现协议级偏差。
	variableHeader := []byte{
		0x00, 0x04, 'M', 'Q', 'T', 'T',
		mqttProtocolLevel311,
		0x02,
		0x00, 0x3c,
	}

	// What: 负载只放 clientID。
	// Why: 文档里当前只要求按机器人 ID 连接，不需要额外认证字段。
	payload := make([]byte, 0, 2+len(clientIDBytes))
	payload = append(payload, byte(len(clientIDBytes)>>8), byte(len(clientIDBytes)))
	payload = append(payload, clientIDBytes...)

	remainingLength := len(variableHeader) + len(payload)
	packet := make([]byte, 0, 2+remainingLength)
	packet = append(packet, mqttPacketTypeConnect, byte(remainingLength))
	packet = append(packet, variableHeader...)
	packet = append(packet, payload...)
	return packet, nil
}

// registerSubscription 统一登记一个订阅目标。
// What: 将 topic/qos/handler 保存到内部表，并在已连接时立即执行一次订阅。
// Why: 这样既覆盖“先注册后连接”的场景，也覆盖“已连接后动态注册”的场景。
func (mc *MQTTClient) registerSubscription(topic string, qos byte, handler pahomqtt.MessageHandler) error {
	if mc == nil || mc.client == nil || handler == nil {
		return nil
	}

	subscription := mqttSubscription{
		topic:   topic,
		qos:     qos,
		handler: handler,
	}

	mc.mu.Lock()
	mc.subscriptions[topic] = subscription
	connected := mc.client.IsConnected()
	client := mc.client
	mc.mu.Unlock()

	// What: 未连接时只注册、不立即报错。
	// Why: 连接建立可能稍后才完成，此时直接返回 not Connected 只会制造误报。
	if !connected {
		return nil
	}

	return mc.subscribeOnce(client, subscription)
}

// resubscribeAll 在连接建立或重连后补齐所有已登记订阅。
// What: 复制一份订阅快照后逐个补订阅。
// Why: 避免持锁期间执行网络 I/O，把 MQTT 回调线程拖住。
func (mc *MQTTClient) resubscribeAll(client pahomqtt.Client) {
	if mc == nil || client == nil {
		return
	}

	mc.mu.Lock()
	subscriptions := make([]mqttSubscription, 0, len(mc.subscriptions))
	for _, subscription := range mc.subscriptions {
		subscriptions = append(subscriptions, subscription)
	}
	mc.mu.Unlock()

	for _, subscription := range subscriptions {
		if err := mc.subscribeOnce(client, subscription); err != nil {
			log.Printf("[Warning] MQTT subscribe %s failed after connect: %v", subscription.topic, err)
		}
	}
}

// subscribeOnce 对单个 topic 执行一次实际订阅。
// What: 将 Subscribe token 的等待与日志收口到单函数。
// Why: 这样所有 topic 的行为一致，后续若要调整超时策略也只改一处。
func (mc *MQTTClient) subscribeOnce(client pahomqtt.Client, subscription mqttSubscription) error {
	token := client.Subscribe(subscription.topic, subscription.qos, subscription.handler)
	token.Wait()
	if err := token.Error(); err != nil {
		return err
	}

	log.Printf("[network] subscribed %s topic", subscription.topic)
	return nil
}

// SubscribeEvent 订阅裁判系统 Event 主题。
// What: 将 protobuf 事件包解码并回调给上层。
// Why: 让上层统一桥接前端事件流，避免前端直接耦合 MQTT 与 protobuf 细节。
func (mc *MQTTClient) SubscribeEvent(onEvent func(event *rmcp.Event)) error {
	if onEvent == nil {
		return nil
	}

	return mc.registerSubscription("Event", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.Event{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode Event payload failed: %v", err)
			return
		}
		onEvent(payload)
	})
}

// SubscribeGameStatus 订阅比赛全局状态主题。
// What: 解码 GameStatus 并回调给上层桥接。
// Why: 顶部比分、阶段、局次与倒计时都必须来自这条官方主题，不能继续让前端用假字段拼接。
func (mc *MQTTClient) SubscribeGameStatus(onStatus func(status *rmcp.GameStatus)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("GameStatus", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.GameStatus{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode GameStatus payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeRobotDynamicStatus 订阅机器人动态状态主题。
// What: 解码 RobotDynamicStatus 并回调给上层桥接。
// Why: 一键买弹需要实时知道 can_remote_ammo，不能让前端靠经济和弹量瞎猜。
func (mc *MQTTClient) SubscribeRobotDynamicStatus(onStatus func(status *rmcp.RobotDynamicStatus)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("RobotDynamicStatus", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.RobotDynamicStatus{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode RobotDynamicStatus payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeGlobalUnitStatus 订阅全局基地与全队血量主题。
// What: 解码 GlobalUnitStatus 并回调给上层桥接。
// Why: 基地血量和双方队伍血量都在这条 topic 上，不能继续靠前端占位或后台模拟。
func (mc *MQTTClient) SubscribeGlobalUnitStatus(onStatus func(status *rmcp.GlobalUnitStatus)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("GlobalUnitStatus", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.GlobalUnitStatus{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode GlobalUnitStatus payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeRobotStaticStatus 订阅机器人固定属性主题。
// What: 解码 RobotStaticStatus 并回调给上层桥接。
// Why: 本机和队友的血量上限都要依赖 max_health，继续写死默认值会让血条长期失真。
func (mc *MQTTClient) SubscribeRobotStaticStatus(onStatus func(status *rmcp.RobotStaticStatus)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("RobotStaticStatus", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.RobotStaticStatus{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode RobotStaticStatus payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeGlobalLogisticsStatus 订阅全局经济与科技状态主题。
func (mc *MQTTClient) SubscribeGlobalLogisticsStatus(onStatus func(status *rmcp.GlobalLogisticsStatus)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("GlobalLogisticsStatus", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.GlobalLogisticsStatus{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode GlobalLogisticsStatus payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeGlobalSpecialMechanism 订阅全局特殊机制状态主题。
func (mc *MQTTClient) SubscribeGlobalSpecialMechanism(onStatus func(status *rmcp.GlobalSpecialMechanism)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("GlobalSpecialMechanism", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.GlobalSpecialMechanism{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode GlobalSpecialMechanism payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeRobotModuleStatus 订阅机器人模块状态主题。
func (mc *MQTTClient) SubscribeRobotModuleStatus(onStatus func(status *rmcp.RobotModuleStatus)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("RobotModuleStatus", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.RobotModuleStatus{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode RobotModuleStatus payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeRobotPosition 订阅机器人实时位置主题。
func (mc *MQTTClient) SubscribeRobotPosition(onStatus func(status *rmcp.RobotPosition)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("RobotPosition", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.RobotPosition{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode RobotPosition payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeRadarInfo 订阅雷达客户端目标信息主题。
func (mc *MQTTClient) SubscribeRadarInfo(onStatus func(status *rmcp.RadarInfoToClient)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("RadarInfoToClient", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.RadarInfoToClient{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode RadarInfoToClient payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeBuff 订阅机器人增益状态主题。
func (mc *MQTTClient) SubscribeBuff(onStatus func(status *rmcp.Buff)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("Buff", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.Buff{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode Buff payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeDeployModeStatus 订阅部署模式同步状态主题。
func (mc *MQTTClient) SubscribeDeployModeStatus(onStatus func(status *rmcp.DeployModeStatusSync)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("DeployModeStatusSync", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.DeployModeStatusSync{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode DeployModeStatusSync payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeAirSupportStatus 订阅空中支援同步主题。
// What: 解码 AirSupportStatusSync 并回调给上层桥接。
// Why: 起飞建议必须依赖裁判系统真实 cost/left_time，不能让前端凭经验硬算窗口。
func (mc *MQTTClient) SubscribeAirSupportStatus(onStatus func(status *rmcp.AirSupportStatusSync)) error {
	if onStatus == nil {
		return nil
	}

	return mc.registerSubscription("AirSupportStatusSync", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.AirSupportStatusSync{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode AirSupportStatusSync payload failed: %v", err)
			return
		}
		onStatus(payload)
	})
}

// SubscribeCustomByteBlock 订阅机器人 0x0310 对应的自定义字节流主题。
// What: 解码 CustomByteBlock 并回调给上层桥接。
// Why: 自定义视频源已经直接建立在这条链上，后端需要持续收到 0x0310 并把原始图像字节交给重组器。
func (mc *MQTTClient) SubscribeCustomByteBlock(onBlock func(block *rmcp.CustomByteBlock)) error {
	if onBlock == nil {
		return nil
	}

	return mc.registerSubscription("CustomByteBlock", 2, func(_ pahomqtt.Client, message pahomqtt.Message) {
		payload := &rmcp.CustomByteBlock{}
		if err := proto.Unmarshal(message.Payload(), payload); err != nil {
			log.Printf("[network] decode CustomByteBlock payload failed: %v", err)
			return
		}
		onBlock(payload)
	})
}

// PubKeyboardMouse 根据新的 RMCP V1.2.0 框架下发键鼠控制
// (Wails 会把前端的数据传至此)
func (mc *MQTTClient) PubKeyboardMouse(
	mouseX, mouseY, mouseZ int32,
	leftBtn, rightBtn, midBtn bool,
	kb uint32) error {

	msg := &rmcp.KeyboardMouseControl{
		MouseX:          mouseX,
		MouseY:          mouseY,
		MouseZ:          mouseZ,
		LeftButtonDown:  leftBtn,
		RightButtonDown: rightBtn,
		MidButtonDown:   midBtn,
		KeyboardValue:   kb,
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	// What: 发布前不额外自造连接状态机，直接交由底层 client 返回真实错误。
	// Why: 这样可以保留最原始的 broker/连接异常信息，方便现场排障。
	token := mc.client.Publish("KeyboardMouseControl", 2, false, data)
	token.Wait()
	return token.Error()
}

// PubCustomControl 封装自定义按键下发
func (mc *MQTTClient) PubCustomControl(customBytes []byte) error {
	msg := &rmcp.CustomControl{Data: customBytes}
	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	token := mc.client.Publish("CustomControl", 2, false, data)
	token.Wait()
	return token.Error()
}

// Close 主动断开 MQTT 连接。
// What: 在应用退出时释放 MQTT 连接资源。
// Why: 避免比赛机反复重启客户端后残留僵尸会话占用 broker 资源。
func (mc *MQTTClient) Close() {
	if mc == nil || mc.client == nil {
		return
	}
	if mc.client.IsConnected() {
		mc.client.Disconnect(250)
	}
}
