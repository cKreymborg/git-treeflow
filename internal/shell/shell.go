package shell

import "fmt"

const bashZshInit = `gtf() {
    local result
    result=$(GTF_WRAPPER=1 command gtf "$@")
    local exit_code=$?
    if [ $exit_code -eq 0 ] && [ -n "$result" ] && [ -d "$result" ]; then
        cd "$result" || return
    elif [ -n "$result" ]; then
        echo "$result"
    fi
    return $exit_code
}`

const fishInit = `function gtf
    set -l result (env GTF_WRAPPER=1 command gtf $argv)
    set -l exit_code $status
    if test $exit_code -eq 0; and test -n "$result"; and test -d "$result"
        cd $result
    else if test -n "$result"
        echo $result
    end
    return $exit_code
end`

func InitScript(shell string) (string, error) {
	switch shell {
	case "zsh", "bash":
		return bashZshInit, nil
	case "fish":
		return fishInit, nil
	default:
		return "", fmt.Errorf("unsupported shell: %s (supported: zsh, bash, fish)", shell)
	}
}
