package upgrade

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Sakurame1/frp-manager/utils"
	"github.com/Sakurame1/frp-manager/utils/logger"
	"github.com/kardianos/service"
)

const upgraderServiceName = "frpp-upgrader"

type StartResult struct {
	Dispatched      bool
	PlanPath        string
	UpgraderService string
}

// Start 鎵ц鍗囩骇锛堥潪 Windows锛氱洿鎺ユ浛鎹笉褰卞搷褰撳墠杩涚▼锛沇indows锛氬惎鍔?worker 瀹屾垚鏇挎崲/鏈嶅姟鎺у埗锛?
func Start(ctx context.Context, opt Options) error {
	_, err := StartWithResult(ctx, opt)
	return err
}

func StartWithResult(ctx context.Context, opt Options) (StartResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	// 鍏抽敭璇婃柇淇℃伅锛氬府鍔╃‘璁も€滃埌搴曟墽琛岀殑鏄摢涓?frp-manager 浜岃繘鍒垛€?
	exePath, _ := os.Executable()
	realExe := exePath
	if rp, err := filepath.EvalSymlinks(exePath); err == nil && len(rp) > 0 {
		realExe = rp
	}
	if abs, err := filepath.Abs(realExe); err == nil {
		realExe = abs
	}

	target, err := resolveTargetPath(opt.TargetPath)
	if err != nil {
		return StartResult{}, err
	}
	opt.TargetPath = target
	if len(strings.TrimSpace(opt.Version)) == 0 {
		opt.Version = "latest"
	}
	if len(opt.WorkDir) == 0 {
		opt.WorkDir = defaultWorkDir()
	}

	unlock, err := lock(opt.WorkDir)
	if err != nil {
		return StartResult{}, err
	}
	defer unlock()

	if err := utils.EnsureDirectoryExists(opt.TargetPath); err != nil {
		return StartResult{}, fmt.Errorf("ensure target directory failed: %w", err)
	}

	downloadURL, err := buildDownloadURL(opt)
	if err != nil {
		return StartResult{}, err
	}

	logger.Logger(ctx).Infof("upgrade: executable=%s, target=%s, restart_service=%v, service_name=%s",
		realExe, opt.TargetPath, opt.RestartService, strings.TrimSpace(opt.ServiceName))
	logger.Logger(ctx).Infof("upgrade: downloading version [%s], url: %s", opt.Version, downloadURL)
	tmpPath, err := utils.DownloadFile(ctx, downloadURL, strings.TrimSpace(opt.HTTPProxy))
	if err != nil {
		return StartResult{}, fmt.Errorf("download failed: %w", err)
	}
	if strings.TrimSpace(opt.DownloadURL) == "" {
		assetName, err := detectAssetName(opt.ClientOnly)
		if err != nil {
			return StartResult{}, err
		}
		if err := verifyOfficialChecksum(ctx, opt, assetName, tmpPath); err != nil {
			return StartResult{}, fmt.Errorf("verify release checksum: %w", err)
		}
		logger.Logger(ctx).Infof("upgrade: SHA256 verified for %s", assetName)
	}

	// stage 鍒扮洰鏍囩洰褰曢檮杩戯紝閬垮厤璺ㄦ枃浠剁郴缁?rename 闂
	staged := stagePathForTarget(opt.TargetPath)
	_ = os.Remove(staged)
	if err := copyFile(tmpPath, staged, 0755); err != nil {
		return StartResult{}, fmt.Errorf("stage new binary failed: %w", err)
	}
	_ = os.Chmod(staged, 0755)

	if err := verifyBinary(staged); err != nil {
		_ = os.Remove(staged)
		return StartResult{}, err
	}

	// Linux 鍦烘櫙锛氶伩鍏嶁€滆繙绋嬪崌绾ч€掑綊渚濊禆鈥濓紙stop/restart frpp 浼氭潃鎺夊悓 unit/cgroup 涓嬬殑鍗囩骇杩涚▼锛?
	// 鍋氭硶锛氬啓鍏ュ浐瀹?plan.json锛岀劧鍚庡惎鍔ㄧ嫭绔嬬殑 upgrader service 鍘?stop鈫掓浛鎹⑩啋start銆?
	if runtime.GOOS == "linux" && opt.RestartService && len(strings.TrimSpace(opt.ServiceName)) > 0 {
		planPath, err := writePlan(opt.WorkDir, opt)
		if err != nil {
			return StartResult{}, err
		}
		logger.Logger(ctx).Infof("upgrade: plan created at %s, dispatching upgrader service: %s", planPath, upgraderServiceName)

		if err := ensureUpgraderService(ctx, planPath); err != nil {
			return StartResult{}, err
		}
		// 寮傛锛氭澶勮繑鍥炲悗 remoteshell 鍙互绔嬪埢寰楀埌鍝嶅簲锛涚湡姝?stop/restart 灏嗙敱 upgrader 瀹屾垚
		return StartResult{Dispatched: true, PlanPath: planPath, UpgraderService: upgraderServiceName}, nil
	}

	// Windows锛氭棤娉曡鐩栨鍦ㄨ繍琛岀殑 exe锛屽洜姝ょ敤鐙珛 worker 鏉ュ仛鏈嶅姟 stop->replace->start
	if runtime.GOOS == "darwin" && opt.RestartService && len(strings.TrimSpace(opt.ServiceName)) > 0 {
		planPath, err := writePlan(opt.WorkDir, opt)
		if err != nil {
			return StartResult{}, err
		}
		if err := spawnWorker(mustExecutablePath(), planPath); err != nil {
			return StartResult{}, fmt.Errorf("start macOS upgrade worker failed: %w", err)
		}
		logger.Logger(ctx).Info("upgrade worker started (macOS); it will stop, replace, and restart the service")
		return StartResult{Dispatched: true, PlanPath: planPath}, nil
	}

	if runtime.GOOS == "windows" {
		planPath, err := writePlan(opt.WorkDir, opt)
		if err != nil {
			return StartResult{}, err
		}
		// 璁?worker 澶嶇敤宸?stage 鐨勬枃浠讹細绾﹀畾 staged 鍥哄畾涓?targetPath+".new"
		if err := spawnWorker(mustExecutablePath(), planPath); err != nil {
			return StartResult{}, fmt.Errorf("start upgrade worker failed: %w", err)
		}
		logger.Logger(ctx).Info("upgrade worker started (windows). it will stop/replace/start service in background if configured")
		return StartResult{Dispatched: true, PlanPath: planPath, UpgraderService: ""}, nil
	}

	// 闈?Windows锛氬綋鍓嶈繘绋嬪彲浠ョ户缁繍琛岋紝鏇挎崲涓嶄細褰卞搷褰撳墠杩愯瀹炰緥
	var backupPath string
	if opt.Backup {
		backupPath, err = backupExisting(opt.TargetPath)
		if err != nil {
			return StartResult{}, err
		}
	}

	if err := replaceFile(staged, opt.TargetPath); err != nil {
		// 鍥炴粴
		if len(backupPath) > 0 {
			_ = replaceFile(backupPath, opt.TargetPath)
		}
		return StartResult{}, fmt.Errorf("replace executable failed: %w", err)
	}

	logger.Logger(ctx).Infof("upgrade: binary replaced successfully: %s", opt.TargetPath)

	if opt.RestartService && len(strings.TrimSpace(opt.ServiceName)) > 0 {
		logger.Logger(ctx).Infof("upgrade: restarting service: %s", opt.ServiceName)
		// 鍙傝€?cmd/frpp/shared/cmd.go锛氫娇鐢?utils.ControlSystemService
		if err := utils.ControlSystemService(opt.ServiceName, opt.ServiceArgs, "restart", func() {}); err != nil {
			// 浜岃繘鍒跺凡缁忔浛鎹㈡垚鍔燂紝杩欓噷鐨勯噸鍚け璐ヤ笉搴斿鑷存暣浣?upgrade 澶辫触锛堝挨鍏舵槸闈?root 鍦烘櫙锛?
			logger.Logger(ctx).WithError(err).Warnf("restart service failed, please restart manually or run with sudo: %s", opt.ServiceName)
			return StartResult{}, nil
		}
	}

	return StartResult{Dispatched: false, PlanPath: "", UpgraderService: ""}, nil
}

func ensureUpgraderService(ctx context.Context, planPath string) error {
	// 缁熶竴浣跨敤 kardianos/service锛氬吋瀹归潪 systemd 鐨?Linux锛坲pstart/sysv/openrc锛?
	// 瀵?systemd锛氶€氳繃 SystemdScript/Restart 閫夐」锛岄伩鍏?Restart=always 瀵艰嚧鏃犻檺閲嶅惎锛?
	// 骞剁敤 ConditionPathExists 闃叉 enable 鍚庡紑鏈鸿嚜鍚瑙﹀彂鍗囩骇銆?

	args := []string{"__upgrade-worker", "--plan", planPath}

	opts := service.KeyValue{
		// 榛樿 systemd 鑴氭湰浼?Restart=always 涓?install 浼?enable锛岃繖浼氬鑷存棤闄愰噸鍚€?
		// 鎴戜滑瀹氬埗涓?oneshot + Restart=no + ConditionPathExists(plan)銆?
		"SystemdScript": systemdUpgraderScript(planPath),
		"Restart":       "no",
	}

	// 淇/瑕嗙洊鏃?unit锛歴top + uninstall锛堝拷鐣ラ敊璇級鍚?install + start
	_ = utils.ControlSystemServiceWithOptions(upgraderServiceName, args, "stop", func() {}, opts)
	_ = utils.ControlSystemServiceWithOptions(upgraderServiceName, args, "uninstall", func() {}, opts)
	if err := utils.ControlSystemServiceWithOptions(upgraderServiceName, args, "install", func() {}, opts); err != nil {
		// 濡傛灉 install 鍥犱负宸插瓨鍦ㄧ瓑鍘熷洜澶辫触锛屽啀灏濊瘯鐩存帴 start
		logger.Logger(ctx).WithError(err).Warn("upgrade: upgrader install failed, try start directly")
	}
	return utils.ControlSystemServiceWithOptions(upgraderServiceName, args, "start", func() {}, opts)
}

func systemdUpgraderScript(planPath string) string {
	// 娉ㄦ剰锛氳繖鏄?kardianos/service 鐨?systemd 妯℃澘鏂囨湰锛屼細琚綋浣?text/template 瑙ｆ瀽锛?
	// 鍥犳鎴戜滑淇濈暀 {{.Path}} / {{.Arguments}} 绛夊崰浣嶇锛屽彧鎶?planPath 鍐欐杩?ConditionPathExists銆?
	return fmt.Sprintf(`[Unit]
Description=frp-manager upgrader (oneshot)
ConditionFileIsExecutable={{.Path|cmdEscape}}
ConditionPathExists=%s

[Service]
Type=oneshot
ExecStart={{.Path|cmdEscape}}{{range .Arguments}} {{.|cmd}}{{end}}
{{if .WorkingDirectory}}WorkingDirectory={{.WorkingDirectory|cmdEscape}}{{end}}
Restart=no

[Install]
WantedBy=multi-user.target
`, planPath)
}

func mustExecutablePath() string {
	p, _ := os.Executable()
	if real, err := filepath.EvalSymlinks(p); err == nil && len(real) > 0 {
		p = real
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return p
}

// RunWorker 鎵ц鍗囩骇璁″垝锛堢粰闅愯棌鍛戒护 __upgrade-worker 璋冪敤锛?
func RunWorker(ctx context.Context, planPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	defer scheduleWorkerCleanup()

	// upgrader service 鍙兘鍦?boot 鎴栨棤 plan 鏃惰鍚姩锛氭鏃剁洿鎺ラ€€鍑猴紝淇濊瘉涓嶄細璇仠 frpp
	if _, err := os.Stat(planPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	p, err := readPlan(planPath)
	if err != nil {
		return err
	}
	opt := p.Options

	target, err := resolveTargetPath(opt.TargetPath)
	if err != nil {
		return err
	}
	opt.TargetPath = target

	// worker 鍋囪 staged 鍥哄畾涓?targetPath+".new"
	staged := stagePathForTarget(opt.TargetPath)
	if err := verifyBinary(staged); err != nil {
		_ = os.Remove(staged)
		_ = writeStatus(opt.WorkDir, false, err.Error())
		return err
	}

	if opt.RestartService && len(strings.TrimSpace(opt.ServiceName)) > 0 {
		// stop锛堣繖涓€姝ヤ細瀵艰嚧 remoteshell 鏂紑锛屼絾 worker 鍦ㄧ嫭绔?service 閲屾墽琛岋紝涓嶄細琚竴璧锋潃鎺夛級
		if err := utils.ControlSystemService(opt.ServiceName, opt.ServiceArgs, "stop", func() {}); err != nil {
			if runtime.GOOS != "windows" {
				_ = writeStatus(opt.WorkDir, false, err.Error())
				return err
			}
			logger.Logger(ctx).WithError(err).Warn("upgrade worker could not stop the Windows service; waiting for the current process to exit")
		}
	}

	// replace锛氬繀椤婚伩鍏嶆墦寮€/truncate target锛堜細瑙﹀彂 ETXTBSY锛夛紝浼樺厛鐢?rename锛坰taged 涓?target 鍚岀洰褰曪級
	if err := replaceStagedToTarget(ctx, staged, opt.TargetPath, opt.Backup); err != nil {
		_ = writeStatus(opt.WorkDir, false, err.Error())
		return err
	}

	if opt.RestartService && len(strings.TrimSpace(opt.ServiceName)) > 0 {
		if err := utils.ControlSystemService(opt.ServiceName, opt.ServiceArgs, "start", func() {}); err != nil {
			_ = writeStatus(opt.WorkDir, false, err.Error())
			return err
		}
	}
	if opt.Relaunch {
		if err := spawnDetached(opt.TargetPath, opt.RelaunchArgs); err != nil {
			_ = writeStatus(opt.WorkDir, false, err.Error())
			return err
		}
	}

	_ = os.Remove(planPath) // 瀹屾垚鍚庢竻鐞?plan锛岄伩鍏嶅紑鏈鸿嚜鍚瑙﹀彂
	_ = writeStatus(opt.WorkDir, true, "ok")
	return nil
}

func replaceStagedToTarget(ctx context.Context, staged, target string, backup bool) error {
	_ = ctx
	if len(staged) == 0 || len(target) == 0 {
		return fmt.Errorf("staged/target is empty")
	}

	if runtime.GOOS == "windows" {
		// Windows锛氶渶瑕佸湪 service stop 鍚庢墠鑳藉姩鐩爣鏂囦欢锛屼笖 rename 瑕嗙洊閫氬父涓嶅厑璁?
		deadline := time.Now().Add(60 * time.Second)
		targetPrepared := false
		for {
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting target file unlock: %s", target)
			}

			if !targetPrepared {
				if backup {
					backupPath := target + ".bak"
					_ = os.Remove(backupPath)
					if err := os.Rename(target, backupPath); err != nil && !os.IsNotExist(err) {
						time.Sleep(500 * time.Millisecond)
						continue
					}
				} else if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
					time.Sleep(500 * time.Millisecond)
					continue
				}
				targetPrepared = true
			}

			if err := os.Rename(staged, target); err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			return nil
		}
	}

	// Unix-like锛歳ename 瑕嗙洊鏄師瀛愭搷浣滐紝涓斾笉鍙楁鍦ㄨ繍琛岀殑鏃?binary 褰卞搷锛堜笉浼氳Е鍙?ETXTBSY锛?
	if backup {
		backupPath := target + ".bak"
		_ = os.Remove(backupPath)
		// 澶囦唤閲囩敤 rename锛岄伩鍏?open/truncate
		_ = os.Rename(target, backupPath)
	}
	if err := os.Rename(staged, target); err != nil {
		return err
	}
	return nil
}
