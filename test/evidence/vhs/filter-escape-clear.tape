Output test/evidence/vhs/filter-escape-clear.gif

Set Shell "zsh"
Set Width 1200
Set Height 650
Set FontSize 18
Set TypingSpeed 80ms
Set Framerate 30
Set CursorBlink false
Set Theme "Catppuccin Mocha"

Hide
Type "rtk go build -o /tmp/runny-filter-escape-clear ./cmd/runny && rtk mkdir -p /tmp/runny-filter-escape-clear-workspace/api /tmp/runny-filter-escape-clear-workspace/web /tmp/runny-filter-escape-clear-workspace/worker && cd /tmp/runny-filter-escape-clear-workspace && clear && /tmp/runny-filter-escape-clear -- echo ok"
Enter
Sleep 2s
Show

Type "/"
Sleep 500ms
Type "api"
Sleep 2s
Enter
Sleep 2s
Escape
Sleep 3s

Hide
Ctrl+C
Type "y"
Sleep 1s
