package main

import (
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/unix"
)

// openSerialInput 根据配置选择“直接读现有串口”或“创建虚拟下位机 PTY”。
// What: 当 serial-link 非空时创建一条新的 PTY 并暴露 slave 路径；否则直接读取现有 serial-port。
// Why: 用户要求本机单程序模拟下位机中转，因此默认必须由桥程序自己提供一个稳定串口入口给上位机连接。
func openSerialInput(cfg simulatorConfig) (serialInput, error) {
	if cfg.serialLink != "" {
		return openPTYInput(cfg.serialLink)
	}
	if cfg.serialPort == "" {
		return serialInput{}, fmt.Errorf("serial-port is empty")
	}

	serialFile, err := openRawSerialPort(cfg.serialPort)
	if err != nil {
		return serialInput{}, err
	}

	return serialInput{
		readFile:         serialFile,
		displayPath:      cfg.serialPort,
		physicalReadPath: cfg.serialPort,
		cleanup: func() {
			_ = serialFile.Close()
		},
	}, nil
}

// openPTYInput 创建一条 PTY，并将 slave 路径暴露到指定链接。
// What: 打开 /dev/ptmx，解锁 slave，并把 slave 路径软链接到调用方指定的位置。
// Why: 这样上位机只需要像连真实串口一样打开一个路径，桥程序则持续从 master 端读取原始 H.264 字节流。
func openPTYInput(linkPath string) (serialInput, error) {
	masterFD, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC, 0)
	if err != nil {
		return serialInput{}, err
	}

	cleanupMaster := func() {
		_ = unix.Close(masterFD)
	}

	var unlock int32
	if _, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(masterFD),
		uintptr(unix.TIOCSPTLCK),
		uintptr(unsafe.Pointer(&unlock)),
	); errno != 0 {
		cleanupMaster()
		return serialInput{}, errno
	}

	slaveIndex, err := unix.IoctlGetInt(masterFD, unix.TIOCGPTN)
	if err != nil {
		cleanupMaster()
		return serialInput{}, err
	}

	slavePath := fmt.Sprintf("/dev/pts/%d", slaveIndex)
	if err := replaceLink(linkPath, slavePath); err != nil {
		cleanupMaster()
		return serialInput{}, err
	}

	masterFile := os.NewFile(uintptr(masterFD), "/dev/ptmx")
	if masterFile == nil {
		_ = os.Remove(linkPath)
		cleanupMaster()
		return serialInput{}, fmt.Errorf("create master PTY file failed")
	}

	return serialInput{
		readFile:         masterFile,
		displayPath:      linkPath,
		physicalReadPath: slavePath,
		cleanup: func() {
			_ = os.Remove(linkPath)
			_ = masterFile.Close()
		},
	}, nil
}

// replaceLink 将调用方指定路径替换为指向 slave 的软链接。
// What: 只允许覆盖已有的普通文件或软链接，不碰目录。
// Why: 本地测试入口通常固定在 /tmp 下，允许幂等覆盖同名旧链接，但不能误删用户真实目录。
func replaceLink(linkPath string, targetPath string) error {
	if linkPath == "" {
		return fmt.Errorf("link path is empty")
	}

	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}

	info, err := os.Lstat(linkPath)
	switch {
	case err == nil:
		if info.IsDir() {
			return fmt.Errorf("refuse to replace directory with PTY link: %s", linkPath)
		}
		if removeErr := os.Remove(linkPath); removeErr != nil {
			return removeErr
		}
	case os.IsNotExist(err):
		// What: 链接路径尚不存在时直接继续创建。
		// Why: 这是首次启动的正常情况，不需要额外分支。
	default:
		return err
	}

	return os.Symlink(targetPath, linkPath)
}
