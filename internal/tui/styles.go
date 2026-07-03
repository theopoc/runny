package tui

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/theopoc/runny/internal/core"
)

type tuiTheme struct {
	fgDefault     color.Color
	fgMuted       color.Color
	fgEmphasis    color.Color
	bgBase        color.Color
	bgSurface     color.Color
	bgSelection   color.Color
	bgRunning     color.Color
	accentPrimary color.Color
	accentWarm    color.Color
	accentCommand color.Color
	success       color.Color
	warning       color.Color
	error         color.Color
	info          color.Color
}

var runnyTheme = tuiTheme{
	fgDefault:     tuiColor("#CBD5E1"),
	fgMuted:       tuiColor("#94A3B8"),
	fgEmphasis:    tuiColor("#F8FAFC"),
	bgBase:        tuiColor("#0F172A"),
	bgSurface:     tuiColor("#111827"),
	bgSelection:   tuiColor("#1E293B"),
	bgRunning:     tuiColor("#172554"),
	accentPrimary: tuiColor("#67E8F9"),
	accentWarm:    tuiColor("#FBBF24"),
	accentCommand: tuiColor("#C4B5FD"),
	success:       tuiColor("#34D399"),
	warning:       tuiColor("#FDE68A"),
	error:         tuiColor("#FB7185"),
	info:          tuiColor("#93C5FD"),
}

var (
	runnyBadgeStyle     = lipgloss.NewStyle().Bold(true).Foreground(runnyTheme.accentCommand)
	headerStyle         = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.bgSurface)
	subtleStyle         = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	panelStyle          = lipgloss.NewStyle().Foreground(runnyTheme.fgDefault)
	panelActiveStyle    = lipgloss.NewStyle().Foreground(runnyTheme.accentPrimary)
	panelTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(runnyTheme.accentPrimary)
	panelInactiveTitle  = lipgloss.NewStyle().Bold(true).Foreground(runnyTheme.fgMuted)
	selectedStyle       = lipgloss.NewStyle().Foreground(runnyTheme.success)
	unselectedStyle     = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	rowActiveStyle      = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.bgSelection).Bold(true)
	rowRunningStyle     = lipgloss.NewStyle().Background(runnyTheme.bgRunning)
	metricIdleStyle     = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	metricQueuedStyle   = lipgloss.NewStyle().Foreground(runnyTheme.info)
	metricRunningStyle  = lipgloss.NewStyle().Foreground(runnyTheme.accentWarm).Bold(true)
	metricSuccessStyle  = lipgloss.NewStyle().Foreground(runnyTheme.success).Bold(true)
	metricFailedStyle   = lipgloss.NewStyle().Foreground(runnyTheme.error).Bold(true)
	commandPromptStyle  = lipgloss.NewStyle().Foreground(runnyTheme.accentPrimary).Background(runnyTheme.bgBase).Bold(true)
	commandDisplayStyle = lipgloss.NewStyle().Foreground(runnyTheme.bgBase).Background(runnyTheme.accentWarm).Bold(true)
	matchStyle          = lipgloss.NewStyle().Foreground(runnyTheme.bgBase).Background(runnyTheme.warning).Bold(true)
	footerKeyStyle      = lipgloss.NewStyle().Foreground(runnyTheme.accentPrimary).Background(runnyTheme.bgSelection).Bold(true)
	overlayTitleStyle   = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.bgSelection).Bold(true)
	sectionStyle        = lipgloss.NewStyle().Foreground(runnyTheme.accentWarm).Bold(true)
	paletteActiveStyle  = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.bgSelection).Bold(true)
	logNumberStyle      = lipgloss.NewStyle().Foreground(runnyTheme.fgMuted)
	logErrorStyle       = lipgloss.NewStyle().Foreground(runnyTheme.error)
	logInfoStyle        = lipgloss.NewStyle().Foreground(runnyTheme.fgDefault)
	noticeStyle         = lipgloss.NewStyle().Foreground(runnyTheme.accentPrimary)
	errorBarStyle       = lipgloss.NewStyle().Foreground(runnyTheme.fgEmphasis).Background(runnyTheme.error).Bold(true)
	warningBarStyle     = lipgloss.NewStyle().Foreground(runnyTheme.bgBase).Background(runnyTheme.warning).Bold(true)
	noticeBarStyle      = lipgloss.NewStyle().Foreground(runnyTheme.bgBase).Background(runnyTheme.accentPrimary).Bold(true)
	statusStyles        = map[core.Status]lipgloss.Style{
		core.StatusIdle:      lipgloss.NewStyle().Foreground(runnyTheme.fgMuted),
		core.StatusQueued:    lipgloss.NewStyle().Foreground(runnyTheme.info),
		core.StatusRunning:   lipgloss.NewStyle().Foreground(runnyTheme.accentWarm).Bold(true),
		core.StatusSucceeded: lipgloss.NewStyle().Foreground(runnyTheme.success).Bold(true),
		core.StatusFailed:    lipgloss.NewStyle().Foreground(runnyTheme.error).Bold(true),
		core.StatusCancelled: lipgloss.NewStyle().Foreground(runnyTheme.fgDefault),
		core.StatusSkipped:   lipgloss.NewStyle().Foreground(runnyTheme.fgMuted),
	}
	footerStyle = lipgloss.NewStyle().Foreground(runnyTheme.fgDefault).Background(runnyTheme.bgSelection)
)

func tuiColor(value string) color.Color {
	if os.Getenv("NO_COLOR") != "" {
		return lipgloss.NoColor{}
	}
	return lipgloss.Color(value)
}
