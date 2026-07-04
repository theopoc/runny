package tui

import (
	"fmt"
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/theopoc/runny/internal/core"
)

type tuiTheme struct {
	fgDefault      color.Color
	fgMuted        color.Color
	fgEmphasis     color.Color
	fgInverse      color.Color
	bgBase         color.Color
	bgCommand      color.Color
	bgSurface      color.Color
	bgElevated     color.Color
	bgHelper       color.Color
	bgSelection    color.Color
	bgRunning      color.Color
	accentPrimary  color.Color
	accentShortcut color.Color
	accentWarm     color.Color
	accentCommand  color.Color
	success        color.Color
	warning        color.Color
	error          color.Color
	info           color.Color
}

const (
	noticeForegroundHex = "#FFFFFF"
	primaryAccentHex    = "#A78BFA"
	footerLabelHex      = "#e0e0e0"
	footerBackgroundHex = "#242f38"
	footerShortcutHex   = "#C4B5FD"
)

var runnyTheme = tuiTheme{
	fgDefault:      tuiColor("#CBD5E1"),
	fgMuted:        tuiColor(footerLabelHex),
	fgEmphasis:     tuiColor("#F8FAFC"),
	fgInverse:      tuiColor("#111827"),
	bgBase:         tuiColor("#10131F"),
	bgCommand:      tuiColor("#05070D"),
	bgSurface:      tuiColor("#191D2B"),
	bgElevated:     tuiColor("#252A3A"),
	bgHelper:       tuiColor(footerBackgroundHex),
	bgSelection:    tuiColor("#A78BFA"),
	bgRunning:      tuiColor("#1E3144"),
	accentPrimary:  tuiColor(primaryAccentHex),
	accentShortcut: tuiColor(footerShortcutHex),
	accentWarm:     tuiColor("#FBBF24"),
	accentCommand:  tuiColor("#C4B5FD"),
	success:        tuiColor("#4ADE80"),
	warning:        tuiColor("#FDE68A"),
	error:          tuiColor("#FB7185"),
	info:           tuiColor("#93C5FD"),
}

var (
	runnyBadgeStyle        = lipgloss.NewStyle().Bold(true).Foreground(runnyTheme.accentCommand)
	headerStyle            = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.bgSurface)
	subtleStyle            = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	panelStyle             = lipgloss.NewStyle().Foreground(runnyTheme.fgDefault)
	panelActiveStyle       = lipgloss.NewStyle().Foreground(runnyTheme.accentCommand)
	panelTitleStyle        = lipgloss.NewStyle().Bold(true).Foreground(runnyTheme.accentCommand)
	panelInactiveTitle     = lipgloss.NewStyle().Bold(true).Foreground(runnyTheme.fgMuted)
	selectedStyle          = lipgloss.NewStyle().Foreground(runnyTheme.success)
	unselectedStyle        = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	rowActiveStyle         = lipgloss.NewStyle().Foreground(runnyTheme.fgInverse).Background(runnyTheme.bgSelection).Bold(true)
	rowSelectedStyle       = rowActiveStyle
	rowActiveSelectedStyle = rowActiveStyle
	rowPartialStyle        = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.bgElevated).Bold(true)
	rowRunningStyle        = lipgloss.NewStyle()
	metricIdleStyle        = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	metricQueuedStyle      = lipgloss.NewStyle().Foreground(runnyTheme.info)
	metricRunningStyle     = lipgloss.NewStyle().Foreground(runnyTheme.accentWarm).Bold(true)
	metricSuccessStyle     = lipgloss.NewStyle().Foreground(runnyTheme.success).Bold(true)
	metricFailedStyle      = lipgloss.NewStyle().Foreground(runnyTheme.error).Bold(true)
	commandPromptStyle     = lipgloss.NewStyle().Foreground(runnyTheme.accentCommand).Background(runnyTheme.bgCommand).Bold(true)
	commandDisplayStyle    = lipgloss.NewStyle().Foreground(runnyTheme.accentCommand).Bold(true)
	commandBarStyle        = lipgloss.NewStyle().Foreground(runnyTheme.accentCommand).Background(runnyTheme.bgCommand).BorderForeground(runnyTheme.accentCommand).Border(lipgloss.NormalBorder(), true, false, true, false)
	matchStyle             = lipgloss.NewStyle().Foreground(runnyTheme.bgBase).Background(runnyTheme.warning).Bold(true)
	overlayTitleStyle      = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.bgSelection).Bold(true)
	sectionStyle           = lipgloss.NewStyle().Foreground(runnyTheme.accentCommand).Bold(true)
	helpKeyStyle           = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Bold(true)
	helpDescStyle          = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	paletteActiveStyle     = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.bgSelection).Bold(true)
	logNumberStyle         = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	logErrorStyle          = lipgloss.NewStyle().Foreground(runnyTheme.error)
	logInfoStyle           = lipgloss.NewStyle().Foreground(runnyTheme.fgDefault)
	noticeStyle            = lipgloss.NewStyle().Foreground(runnyTheme.accentPrimary)
	errorBarStyle          = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.error).Bold(true)
	warningBarStyle        = lipgloss.NewStyle().Foreground(runnyTheme.bgBase).Background(runnyTheme.warning).Bold(true)
	noticeBarStyle         = lipgloss.NewStyle().Foreground(tuiColor(noticeForegroundHex)).Background(runnyTheme.accentPrimary).Bold(true)
	statusStyles           = map[core.Status]lipgloss.Style{
		core.StatusIdle:      lipgloss.NewStyle().Foreground(runnyTheme.fgMuted),
		core.StatusQueued:    lipgloss.NewStyle().Foreground(runnyTheme.info),
		core.StatusRunning:   lipgloss.NewStyle().Foreground(runnyTheme.accentWarm).Bold(true),
		core.StatusSucceeded: lipgloss.NewStyle().Foreground(runnyTheme.success).Bold(true),
		core.StatusFailed:    lipgloss.NewStyle().Foreground(runnyTheme.error).Bold(true),
		core.StatusCancelled: lipgloss.NewStyle().Foreground(runnyTheme.fgDefault),
		core.StatusSkipped:   lipgloss.NewStyle().Foreground(runnyTheme.fgMuted),
	}
	dashboardWidgetStyle = lipgloss.NewStyle().Foreground(runnyTheme.fgDefault).Background(runnyTheme.bgElevated)
	dashboardLabelStyle  = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted).Background(runnyTheme.bgElevated)
	folderIconStyle      = lipgloss.NewStyle().Foreground(runnyTheme.info).Bold(true)
	folderPathStyle      = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	folderNameStyle      = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis)
	treeGuideStyle       = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	statusHeaderStyle    = lipgloss.NewStyle().Foreground(runnyTheme.accentCommand).Bold(true)
)

func tuiColor(value string) color.Color {
	if noColorEnabled() {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(value)
}

func noColorEnabled() bool {
	return os.Getenv("NO_COLOR") != ""
}

func ansiForegroundHex(value string) string {
	return ansiHex("38", value)
}

func ansiBackgroundHex(value string) string {
	return ansiHex("48", value)
}

func ansiHex(kind string, value string) string {
	var r, g, b int
	if _, err := fmt.Sscanf(value, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[%s;2;%d;%d;%dm", kind, r, g, b)
}
