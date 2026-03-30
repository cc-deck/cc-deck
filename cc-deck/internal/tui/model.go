package tui

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/cc-deck/cc-deck/internal/env"
)

// viewType identifies the current TUI view.
type viewType int

const (
	viewList viewType = iota
	viewCreate
	viewHelp
)

// model is the root bubbletea model.
type model struct {
	opts    Options
	view    viewType
	envs    []envRow
	cursor  int
	width   int
	height  int
	store   *env.FileStateStore
	defs    *env.DefinitionStore
	err     error
	message string // transient status message

	// Sub-models
	create  createModel
	confirm *confirmModel
	help    helpModel

	// Polling
	pollLocalInterval     time.Duration
	pollContainerInterval time.Duration
}

// messages
type envListMsg struct {
	envs []envRow
}

type errMsg struct {
	err error
}

type statusMsg struct {
	message string
}

type clearMessageMsg struct{}

func newModel(opts Options) model {
	return model{
		opts:                  opts,
		view:                  viewList,
		store:                 env.NewStateStore(""),
		defs:                  env.NewDefinitionStore(""),
		pollLocalInterval:     opts.PollLocal,
		pollContainerInterval: opts.PollContainer,
		create:                newCreateModel(),
		help:                  newHelpModel(),
	}
}

func newProgram(m model) *tea.Program {
	return tea.NewProgram(m, tea.WithAltScreen())
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		pollEnvs(m.store, m.defs),
		tickPoll(m.pollLocalInterval),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case envListMsg:
		m.envs = msg.envs
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case statusMsg:
		m.message = msg.message
		return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
			return clearMessageMsg{}
		})

	case clearMessageMsg:
		m.message = ""
		return m, nil

	case tickMsg:
		return m, pollEnvs(m.store, m.defs)

	case tea.ResumeMsg:
		return m, pollEnvs(m.store, m.defs)
	}

	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Confirmation dialog takes priority.
	if m.confirm != nil {
		return m.handleConfirmKey(msg)
	}

	// Help overlay takes priority.
	if m.view == viewHelp {
		return m.handleHelpKey(msg)
	}

	// Global keys.
	switch {
	case matchKey(msg, defaultGlobalKeys.Quit):
		return m, tea.Quit
	case matchKey(msg, defaultGlobalKeys.Help):
		m.view = viewHelp
		return m, nil
	case matchKey(msg, defaultGlobalKeys.Refresh):
		return m, pollEnvs(m.store, m.defs)
	}

	// View-specific keys.
	switch m.view {
	case viewList:
		return m.handleListKey(msg)
	case viewCreate:
		return m.handleCreateKey(msg)
	}

	return m, nil
}

func (m model) handleListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, defaultListKeys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case matchKey(msg, defaultListKeys.Down):
		if m.cursor < len(m.envs)-1 {
			m.cursor++
		}
		return m, nil

	case matchKey(msg, defaultListKeys.Top):
		m.cursor = 0
		return m, nil

	case matchKey(msg, defaultListKeys.Bottom):
		if len(m.envs) > 0 {
			m.cursor = len(m.envs) - 1
		}
		return m, nil

	case matchKey(msg, defaultListKeys.Attach):
		return m.doAttach()

	case matchKey(msg, defaultListKeys.New):
		m.view = viewCreate
		m.create = newCreateModel()
		return m, nil

	case matchKey(msg, defaultListKeys.Start):
		return m.doStart()

	case matchKey(msg, defaultListKeys.Stop):
		return m.doStop()

	case matchKey(msg, defaultListKeys.Delete):
		return m.doDelete()

	case matchKey(msg, defaultGlobalKeys.Escape):
		return m, nil
	}

	return m, nil
}

func (m model) handleCreateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, defaultCreateKeys.Cancel):
		m.view = viewList
		return m, nil
	case matchKey(msg, defaultCreateKeys.Confirm):
		return m.doCreate()
	}

	// Delegate to create model for field navigation and input.
	var cmd tea.Cmd
	m.create, cmd = m.create.Update(msg)
	return m, cmd
}

func (m model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, defaultGlobalKeys.Escape),
		matchKey(msg, defaultGlobalKeys.Help),
		matchKey(msg, defaultGlobalKeys.Quit):
		m.view = viewList
		return m, nil
	}
	return m, nil
}

func (m model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case matchKey(msg, defaultGlobalKeys.Escape):
		m.confirm = nil
		return m, nil
	}

	var cmd tea.Cmd
	confirm, c := m.confirm.Update(msg)
	m.confirm = &confirm
	cmd = c

	if m.confirm.confirmed {
		name := m.confirm.targetName
		m.confirm = nil
		return m, m.executeDelete(name)
	}

	return m, cmd
}

func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	switch m.view {
	case viewHelp:
		return m.help.View(m.width, m.height)
	case viewCreate:
		return m.create.View(m.width, m.height)
	default:
		return m.viewList()
	}
}

// selectedEnv returns the currently selected environment row, or nil.
func (m model) selectedEnv() *envRow {
	if m.cursor >= 0 && m.cursor < len(m.envs) {
		return &m.envs[m.cursor]
	}
	return nil
}

// doAttach handles the Enter key on a selected environment.
func (m model) doAttach() (tea.Model, tea.Cmd) {
	sel := m.selectedEnv()
	if sel == nil {
		return m, nil
	}

	if sel.state == "stopped" || sel.state == "not created" {
		m.message = fmt.Sprintf("Environment %q is %s. Start it first.", sel.name, sel.state)
		return m, nil
	}

	if sel.state != "running" {
		m.message = fmt.Sprintf("Cannot attach: environment %q is %s", sel.name, sel.state)
		return m, nil
	}

	attachCmd := buildAttachCommand(sel.name, sel.envType)
	if attachCmd == nil {
		m.message = "Cannot determine attach command for this environment type"
		return m, nil
	}

	return m, tea.ExecProcess(attachCmd, func(err error) tea.Msg {
		if err != nil {
			return errMsg{err: fmt.Errorf("attach failed: %w", err)}
		}
		return tea.ResumeMsg{}
	})
}

// buildAttachCommand constructs the exec.Cmd for attaching to an environment.
func buildAttachCommand(name, envType string) *exec.Cmd {
	sessionName := "cc-deck-" + name

	switch envType {
	case "local":
		return exec.Command("zellij", "attach", sessionName)
	case "container", "compose":
		containerName := "cc-deck-" + name
		return exec.Command("podman", "exec", "-it", containerName, "zellij", "attach", "--create", "cc-deck")
	default:
		return nil
	}
}

// doStart handles the S key to start a stopped environment.
func (m model) doStart() (tea.Model, tea.Cmd) {
	sel := m.selectedEnv()
	if sel == nil {
		return m, nil
	}
	if sel.state != "stopped" {
		m.message = fmt.Sprintf("Environment %q is not stopped (state: %s)", sel.name, sel.state)
		return m, nil
	}
	name := sel.name
	return m, func() tea.Msg {
		e, err := resolveEnv(name, m.store, m.defs)
		if err != nil {
			return errMsg{err: err}
		}
		if err := e.Start(context.Background()); err != nil {
			return errMsg{err: fmt.Errorf("start %q: %w", name, err)}
		}
		return statusMsg{message: fmt.Sprintf("Environment %q started", name)}
	}
}

// doStop handles the X key to stop a running environment.
func (m model) doStop() (tea.Model, tea.Cmd) {
	sel := m.selectedEnv()
	if sel == nil {
		return m, nil
	}
	if sel.state != "running" {
		m.message = fmt.Sprintf("Environment %q is not running (state: %s)", sel.name, sel.state)
		return m, nil
	}
	name := sel.name
	return m, func() tea.Msg {
		e, err := resolveEnv(name, m.store, m.defs)
		if err != nil {
			return errMsg{err: err}
		}
		if err := e.Stop(context.Background()); err != nil {
			return errMsg{err: fmt.Errorf("stop %q: %w", name, err)}
		}
		return statusMsg{message: fmt.Sprintf("Environment %q stopped", name)}
	}
}

// doDelete handles the d key by showing a confirmation dialog.
func (m model) doDelete() (tea.Model, tea.Cmd) {
	sel := m.selectedEnv()
	if sel == nil {
		return m, nil
	}
	confirm := newConfirmModel(sel.name)
	m.confirm = &confirm
	return m, nil
}

// executeDelete runs the actual delete operation after confirmation.
func (m model) executeDelete(name string) tea.Cmd {
	return func() tea.Msg {
		e, err := resolveEnv(name, m.store, m.defs)
		if err != nil {
			return errMsg{err: err}
		}
		if err := e.Delete(context.Background(), true); err != nil {
			return errMsg{err: fmt.Errorf("delete %q: %w", name, err)}
		}
		return statusMsg{message: fmt.Sprintf("Environment %q deleted", name)}
	}
}

// doCreate handles the creation wizard submission.
func (m model) doCreate() (tea.Model, tea.Cmd) {
	name, envType := m.create.values()
	if name == "" {
		m.create.err = "Name is required"
		return m, nil
	}

	m.view = viewList
	return m, func() tea.Msg {
		e, err := env.NewEnvironment(env.EnvironmentType(envType), name, m.store, m.defs)
		if err != nil {
			return errMsg{err: fmt.Errorf("create %q: %w", name, err)}
		}
		opts := env.CreateOpts{}
		if m.create.image != "" {
			opts.Image = m.create.image
		}
		if err := e.Create(context.Background(), opts); err != nil {
			return errMsg{err: fmt.Errorf("create %q: %w", name, err)}
		}
		return statusMsg{message: fmt.Sprintf("Environment %q created", name)}
	}
}

// resolveEnv finds an environment by name across v1 records and v2 instances.
func resolveEnv(name string, store *env.FileStateStore, defs *env.DefinitionStore) (env.Environment, error) {
	if record, err := store.FindByName(name); err == nil {
		return env.NewEnvironment(record.Type, name, store, defs)
	}
	if inst, err := store.FindInstanceByName(name); err == nil {
		instType := env.EnvironmentTypeContainer
		if inst.Type != "" {
			instType = inst.Type
		} else if inst.Compose != nil {
			instType = env.EnvironmentTypeCompose
		}
		return env.NewEnvironment(instType, name, store, defs)
	}
	return nil, fmt.Errorf("environment %q not found", name)
}

// matchKey checks if a key message matches a key binding.
func matchKey(msg tea.KeyMsg, binding key.Binding) bool {
	for _, k := range binding.Keys() {
		if msg.String() == k {
			return true
		}
	}
	return false
}
