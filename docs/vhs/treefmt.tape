Require rsync
Require treefmt

Set Shell zsh
Set FontSize 11

Set Theme "Catppuccin Mocha"

Set Width 320
Set Height 220
Set Padding 20

Hide
Type "TEMP_DIR=$(mktemp -d) && rsync -av $PRJ_ROOT/test/examples/ $TEMP_DIR && pushd $TEMP_DIR && clear"
Enter
Type "find . -type f -exec sh -c 'echo "  " >> {}' \;"
Enter
Type "alias treefmt='treefmt --allow-missing-formatter' && clear"
Enter
Show

Type "treefmt"
Sleep 1s
Enter

Sleep 3s
Enter
Enter
Sleep 1s

Type "treefmt"
Enter

Sleep 5s

Hide
Type "popd && rm -rf $TEMP_DIR"
Enter