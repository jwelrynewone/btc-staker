package e2etest

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/babylonchain/babylon/types"
	"github.com/btcsuite/btcd/btcec/v2"
)

func baseDirBabylondir() (string, error) {
	tempPath := os.TempDir()

	tempName, err := os.MkdirTemp(tempPath, "zBabylonTestStaker")
	if err != nil {
		return "", err
	}

	err = os.Chmod(tempName, 0755)

	if err != nil {
		return "", err
	}

	return tempName, nil
}

type babylonNode struct {
	cmd     *exec.Cmd
	pidFile string
	dataDir string
}

func newBabylonNode(dataDir string, cmd *exec.Cmd) *babylonNode {
	return &babylonNode{
		dataDir: dataDir,
		cmd:     cmd,
	}
}

func (n *babylonNode) start() error {
	if err := n.cmd.Start(); err != nil {
		return err
	}

	pid, err := os.Create(filepath.Join(n.dataDir,
		fmt.Sprintf("%s.pid", "config")))
	if err != nil {
		return err
	}

	n.pidFile = pid.Name()
	if _, err = fmt.Fprintf(pid, "%d\n", n.cmd.Process.Pid); err != nil {
		return err
	}

	if err := pid.Close(); err != nil {
		return err
	}

	return nil
}

func (n *babylonNode) stop() (err error) {
	if n.cmd == nil || n.cmd.Process == nil {
		// return if not properly initialized
		// or error starting the process
		return nil
	}

	defer func() {
		err = n.cmd.Wait()
	}()

	if runtime.GOOS == "windows" {
		return n.cmd.Process.Signal(os.Kill)
	}
	return n.cmd.Process.Signal(os.Interrupt)
}

func (n *babylonNode) cleanup() error {
	if n.pidFile != "" {
		if err := os.Remove(n.pidFile); err != nil {
			log.Printf("unable to remove file %s: %v", n.pidFile,
				err)
		}
	}

	dirs := []string{
		n.dataDir,
	}
	var err error
	for _, dir := range dirs {
		if err = os.RemoveAll(dir); err != nil {
			log.Printf("Cannot remove dir %s: %v", dir, err)
		}
	}
	return nil
}

func (n *babylonNode) shutdown() error {
	if err := n.stop(); err != nil {
		return err
	}
	if err := n.cleanup(); err != nil {
		return err
	}
	return nil
}

type BabylonNodeHandler struct {
	babylonNode *babylonNode
}

func NewBabylonNodeHandler(
	coventantQUorum int,
	covenantPk1 *btcec.PublicKey,
	covenantPk2 *btcec.PublicKey,
	covenantPk3 *btcec.PublicKey,
	slashingAddress string,
	baseHeaderHex string,
) (*BabylonNodeHandler, error) {
	testDir, err := baseDirBabylondir()
	if err != nil {
		return nil, err
	}

	quorumString := strconv.Itoa(coventantQUorum)
	pubBabylon1 := types.NewBIP340PubKeyFromBTCPK(covenantPk1)
	pubBabylon2 := types.NewBIP340PubKeyFromBTCPK(covenantPk2)
	pubBabylon3 := types.NewBIP340PubKeyFromBTCPK(covenantPk3)

	initTestnetCmd := exec.Command(
		"babylond",
		"testnet",
		"--v=1",
		fmt.Sprintf("--output-dir=%s", testDir),
		"--starting-ip-address=192.168.10.2",
		"--keyring-backend=test",
		"--chain-id=chain-test",
		"--btc-finalization-timeout=4",
		"--btc-confirmation-depth=2",
		"--btc-network=regtest",
		fmt.Sprintf("--slashing-address=%s", slashingAddress),
		fmt.Sprintf("--btc-base-header=%s", baseHeaderHex),
		"--additional-sender-account",
		fmt.Sprintf("--covenant-quorum=%s", quorumString),
		fmt.Sprintf("--epoch-interval=%d", 5),
		fmt.Sprintf("--covenant-pks=%s,%s,%s", pubBabylon1.MarshalHex(), pubBabylon2.MarshalHex(), pubBabylon3.MarshalHex()),
	)

	var stderr bytes.Buffer
	initTestnetCmd.Stderr = &stderr

	err = initTestnetCmd.Run()

	if err != nil {
		// remove the testDir if this fails
		_ = os.RemoveAll(testDir)
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return nil, err
	}

	nodeDataDir := filepath.Join(testDir, "node0", "babylond")

	f, err := os.Create(filepath.Join(testDir, "babylon.log"))

	if err != nil {
		return nil, err
	}

	startCmd := exec.Command(
		"babylond",
		"start",
		fmt.Sprintf("--home=%s", nodeDataDir),
		"--log_level=debug",
	)

	startCmd.Stdout = f

	return &BabylonNodeHandler{
		babylonNode: newBabylonNode(testDir, startCmd),
	}, nil
}

func (w *BabylonNodeHandler) Start() error {
	if err := w.babylonNode.start(); err != nil {
		// try to cleanup after start error, but return original error
		_ = w.babylonNode.cleanup()
		return err
	}
	return nil
}

func (w *BabylonNodeHandler) Stop() error {
	if err := w.babylonNode.shutdown(); err != nil {
		return err
	}

	return nil
}

func (w *BabylonNodeHandler) GetNodeDataDir() string {
	dir := filepath.Join(w.babylonNode.dataDir, "node0", "babylond")
	return dir
}
