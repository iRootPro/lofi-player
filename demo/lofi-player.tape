# vhs script for the README demo. Regenerate with:
#
#   vhs demo/lofi-player.tape
#
# This intentionally does NOT press Space (avoids relying on a live
# stream during recording — the resulting GIF would stall on buffering
# and leak the recorder's machine into network logs).

Output demo/lofi-player.gif

Set Shell zsh
Set FontSize 14
Set Width 1200
Set Height 720
Set Padding 24
Set Theme "TokyoNight"
Set TypingSpeed 60ms

Hide
Type "go run ."
Enter
Sleep 2s
Show

# settle on the default station list
Sleep 1500ms

# walk down the list
Down@500ms 3
Sleep 800ms

# walk back up to the top
Up@400ms 3
Sleep 1s

# open ambient mixer, tweak a couple of channels
Type "x"
Sleep 1200ms
Down@350ms 2
Sleep 400ms
Type "lllllll"
Sleep 600ms
Down@350ms 1
Type "llllll"
Sleep 800ms
Escape
Sleep 1s

# cycle the theme
Type "t"
Sleep 1500ms
Type "t"
Sleep 1500ms
Type "t"
Sleep 1500ms
Type "t"
Sleep 1500ms

# show mini mode
Type "m"
Sleep 2s
Type "m"
Sleep 1500ms

# clean exit
Type "q"
Sleep 800ms
