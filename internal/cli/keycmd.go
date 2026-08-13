package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/166176/harness/internal/secret"
	"golang.org/x/term"
)

// cmdKey 分发 key 子命令（SPEC §3.2：set/status/clear）。
func (a *app) cmdKey(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(a.stderr, "key 需要子命令：set|status|clear")
		return ExitConfig
	}
	switch args[0] {
	case "set":
		return a.keySet()
	case "status":
		return a.keyStatus()
	case "clear":
		return a.keyClear()
	default:
		fmt.Fprintf(a.stderr, "未知 key 子命令 %q（可用：set|status|clear）\n", args[0])
		return ExitConfig
	}
}

// keySet 终端隐藏录入（x/term ReadPassword）后写入 keyring（服务名 gavel）；
// 非 TTY 环境报错并引导替代方案。
func (a *app) keySet() int {
	f, ok := a.stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		fmt.Fprintln(a.stderr, "key set 需要交互式终端（TTY）隐藏录入，请直接在终端运行 `gavel key set`")
		fmt.Fprintln(a.stderr, "非交互环境可改用 .env 回退：向 .env 写入 GAVEL_API_KEY=<key>（明文风险见 SPEC §9）")
		return ExitConfig
	}
	fmt.Fprint(a.stdout, "输入 API key（隐藏录入）: ")
	b, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(a.stdout)
	if err != nil {
		fmt.Fprintf(a.stderr, "读取输入失败：%v\n", err)
		return ExitConfig
	}
	key := strings.TrimSpace(string(b))
	if key == "" {
		fmt.Fprintln(a.stderr, "key 不能为空")
		return ExitConfig
	}
	if err := a.keyring.Set(key); err != nil {
		fmt.Fprintf(a.stderr, "写入 keyring 失败：%v\n", err)
		fmt.Fprintln(a.stderr, "keyring 不可用（如 Linux 无 dbus）时可用 .env 回退：GAVEL_API_KEY=<key>")
		return ExitConfig
	}
	fmt.Fprintf(a.stdout, "已写入 %s（指纹 %s）\n", a.keyring.Name(), secret.Fingerprint(key))
	return ExitOK
}

// keyStatus 只显示 provider 名/掩码/指纹，绝不回显明文（SPEC §3.2）。
// 存储位置按 keyring → .env 顺序探测。
func (a *app) keyStatus() int {
	if key, err := a.keyring.Get(); err == nil {
		printKeyStatus(a.stdout, a.keyring.Name(), key)
		return ExitOK
	}
	if key, err := a.dotenv.Get(); err == nil {
		printKeyStatus(a.stdout, a.dotenv.Name(), key)
		return ExitOK
	}
	fmt.Fprintln(a.stdout, "未配置 API key（运行 `gavel key set` 录入，或提供 .env）")
	return ExitOK
}

func printKeyStatus(w io.Writer, provider, key string) {
	fmt.Fprintf(w, "provider: %s\n", provider)
	fmt.Fprintf(w, "mask: %s\n", secret.Mask(key))
	fmt.Fprintf(w, "fingerprint: %s\n", secret.Fingerprint(key))
}

// keyClear 删除 keyring 条目并提示检查 .env（SPEC §3.2）。
func (a *app) keyClear() int {
	if err := a.keyring.Clear(); err != nil && err != secret.ErrNotFound {
		fmt.Fprintf(a.stderr, "清除 keyring 失败：%v\n", err)
		return ExitConfig
	}
	fmt.Fprintln(a.stdout, "已清除 keyring 条目（服务名 gavel）")
	fmt.Fprintln(a.stdout, "提示：如曾使用 .env 回退，请一并删除其中的 GAVEL_API_KEY")
	return ExitOK
}
