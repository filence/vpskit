package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type menuExecutor func([]string, string, string) error

func runMenu(arguments []string, version, publicKeyBase64 string) error {
	if len(arguments) != 0 {
		return errors.New("usage: vpskit menu")
	}
	return runMenuWithIO(os.Stdin, os.Stdout, version, publicKeyBase64, Run)
}

func runMenuWithIO(input io.Reader, output io.Writer, version, publicKeyBase64 string, execute menuExecutor) error {
	reader := bufio.NewReader(input)
	for {
		printManagementMenu(output, version)
		choice, err := readMenuValue(reader, output, "请选择操作 [0-17]：")
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(output, "输入已结束，退出管理菜单。")
			return nil
		}
		if err != nil {
			return err
		}
		if choice == "0" {
			fmt.Fprintln(output, "已退出VPSKit管理菜单。")
			return nil
		}

		arguments, cancelled, err := menuArguments(reader, output, choice)
		if errors.Is(err, io.EOF) {
			fmt.Fprintln(output, "输入已结束，退出管理菜单。")
			return nil
		}
		if err != nil {
			return err
		}
		if cancelled {
			fmt.Fprintln(output, "操作已取消，未修改VPSKit。")
			continue
		}
		if len(arguments) == 0 {
			fmt.Fprintln(output, "无效选择，请重新输入。")
			continue
		}

		fmt.Fprintln(output, "\n正在执行，请稍候……")
		if err := execute(arguments, version, publicKeyBase64); err != nil {
			fmt.Fprintf(output, "操作失败：%v\n", err)
		} else {
			fmt.Fprintln(output, "命令执行结束，请以以上status字段为准。")
		}
		fmt.Fprintln(output)
	}
}

func printManagementMenu(output io.Writer, version string) {
	fmt.Fprintf(output, `
VPSKit 中文管理菜单 (%s)
  1. 查看运行状态
  2. 运行完整诊断
  3. 导出客户端配置ZIP
  4. 显示客户端二维码
  5. 扫描REALITY目标
  6. 更换REALITY目标
  7. 创建配置备份
  8. 查看证书状态
  9. 立即续期证书
 10. 恢复中断的变更
 11. 查看节点元数据
 12. 修改节点显示名
 13. 查看系统与网络摘要
 14. 预览安全清理计划
 15. 导出脱敏诊断包
 16. 预览状态迁移计划

 17. 查看Hysteria2 UDP缓冲状态
  0. 退出
`, version)
}

func menuArguments(reader *bufio.Reader, output io.Writer, choice string) ([]string, bool, error) {
	switch choice {
	case "1":
		return []string{"status"}, false, nil
	case "2":
		return []string{"doctor"}, false, nil
	case "3":
		return []string{"export", "--format", "bundle"}, false, nil
	case "4":
		return []string{"export", "--format", "qr"}, false, nil
	case "5":
		targets, err := readMenuValue(reader, output, "目标域名（多个用逗号分隔，留空扫描内置候选）：")
		if err != nil {
			return nil, false, err
		}
		arguments := []string{"reality", "scan"}
		if targets != "" {
			arguments = append(arguments, "--targets", targets)
		}
		return arguments, false, nil
	case "6":
		target, err := readMenuValue(reader, output, "新的REALITY目标域名：")
		if err != nil {
			return nil, false, err
		}
		if target == "" {
			return nil, true, nil
		}
		fmt.Fprintln(output, "系统将先验证目标，再自动备份、切换配置并增加配置修订号。")
		confirmation, err := readMenuValue(reader, output, "输入 APPLY 确认更换：")
		if err != nil {
			return nil, false, err
		}
		if confirmation != "APPLY" {
			return nil, true, nil
		}
		return []string{"instance", "modify", "reality", "--reality-server-name", strings.ToLower(target)}, false, nil
	case "7":
		return []string{"backup"}, false, nil
	case "8":
		return []string{"cert", "status"}, false, nil
	case "9":
		fmt.Fprintln(output, "续期会联系当前ACME CA和DNS提供商，并在成功后重载服务。")
		confirmation, err := readMenuValue(reader, output, "输入 RENEW 确认续期：")
		if err != nil {
			return nil, false, err
		}
		if confirmation != "RENEW" {
			return nil, true, nil
		}
		return []string{"cert", "renew"}, false, nil
	case "10":
		fmt.Fprintln(output, "恢复命令只处理VPSKit记录的未完成事务。")
		confirmation, err := readMenuValue(reader, output, "输入 RECOVER 确认恢复：")
		if err != nil {
			return nil, false, err
		}
		if confirmation != "RECOVER" {
			return nil, true, nil
		}
		return []string{"recover"}, false, nil
	case "11":
		return []string{"node", "show"}, false, nil
	case "12":
		name, err := readMenuValue(reader, output, "新的节点显示名（不含协议后缀）：")
		if err != nil {
			return nil, false, err
		}
		if name == "" {
			return nil, true, nil
		}
		fmt.Fprintln(output, "修改后会重新生成客户端产物并增加配置修订号。")
		confirmation, err := readMenuValue(reader, output, "输入 APPLY 确认修改：")
		if err != nil {
			return nil, false, err
		}
		if confirmation != "APPLY" {
			return nil, true, nil
		}
		return []string{"node", "modify", "--display-name", name}, false, nil
	case "13":
		return []string{"system", "inspect"}, false, nil
	case "14":
		return []string{"cleanup", "plan"}, false, nil
	case "15":
		return []string{"support", "bundle"}, false, nil
	case "16":
		return []string{"migrate", "plan"}, false, nil
	case "17":
		return []string{"hysteria2", "udp-buffer", "status"}, false, nil
	default:
		return nil, false, nil
	}
}

func readMenuValue(reader *bufio.Reader, output io.Writer, prompt string) (string, error) {
	fmt.Fprint(output, prompt)
	line, err := reader.ReadString('\n')
	value := strings.TrimSpace(line)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if errors.Is(err, io.EOF) && value == "" {
		return "", io.EOF
	}
	return value, nil
}
