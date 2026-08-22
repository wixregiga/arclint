#!/usr/bin/env bash
# Create a GitHub issue from a local issue form, then a linked branch.
#
# GitHub's GraphQL `repository.issueTemplates` only exposes markdown
# templates, so `gh issue create --template` cannot see the YAML issue
# forms in .github/ISSUE_TEMPLATE. This renders the form locally into the
# same `### Label` shape the web form produces. The editor is opened here
# rather than through `gh --editor`, which refuses to run whenever gh's
# stdout is captured.
set -euo pipefail

readonly DEFAULT_ASSIGNEE=wixregiga
readonly DEFAULT_EDITOR_CMD=code-insiders
readonly TEMPLATE_DIR=.github/ISSUE_TEMPLATE

# Global so the EXIT trap can still see it once main has returned.
body_file=""
cleanup() {
	[[ -n $body_file ]] && rm -f "$body_file"
	return 0
}
trap cleanup EXIT

fail() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

usage() {
	cat <<'EOF'
Usage: scripts/new-issue.sh [options] [premise]

Creates a GitHub issue from .github/ISSUE_TEMPLATE, opens it in an editor
for you to fill in, then creates a branch linked to that issue.

Options:
  -t, --type <bug|feature>   Issue form to use (default: feature)
  -b, --branch <name>        Branch to create (prompted when omitted)
  -e, --editor <command>     Editor for the issue body (default: code-insiders)
  -a, --assignee <login>     Issue assignee (default: wixregiga)
      --base <branch>        Branch to fork from (default: repo default branch)
      --no-checkout          Create the branch without checking it out
  -y, --yes                  Skip the confirmation prompt
  -n, --dry-run              Print what would happen; change nothing
  -h, --help                 Show this help

Examples:
  scripts/new-issue.sh
  scripts/new-issue.sh -t bug -b fix/rule-selector "selector drops nested globs"
  scripts/new-issue.sh -n -t feature "add a json output mode"
EOF
}

have() { command -v "$1" >/dev/null 2>&1; }

require_tools() {
	local missing=()
	local tool
	for tool in gh git gum yq jq; do
		have "$tool" || missing+=("$tool")
	done
	if ((${#missing[@]})); then
		fail "missing required tools: ${missing[*]}"
	fi
	gh auth status >/dev/null 2>&1 || fail "gh is not authenticated; run: gh auth login"
}

# Mirrors GitHub's own form rendering: one '### Label' per field, with the
# field description kept as an HTML comment so it stays invisible once posted.
render_body() {
	local file=$1
	yq -o=json '.' "$file" | jq -r '
		[ .body[]
		  | if .type == "markdown" then
		      (.attributes.value // "")
		    elif .type == "checkboxes" then
		      "### " + (.attributes.label // .id // "Field")
		      + "\n\n"
		      + ([ (.attributes.options // [])[] | "- [ ] " + (.label // "") ] | join("\n"))
		    else
		      "### " + (.attributes.label // .id // "Field")
		      + "\n\n<!-- "
		      + (.attributes.description // .attributes.placeholder // "")
		      + (if (.validations.required // false) then " (required)" else " (optional)" end)
		      + (if .type == "dropdown" then
		           " Options: " + ((.attributes.options // []) | join(", "))
		         else "" end)
		      + " -->"
		      + (if (.attributes.render // "") != "" then
		           "\n\n```" + .attributes.render + "\n\n```"
		         else "" end)
		    end
		] | join("\n\n")
	'
}

template_labels() {
	yq -o=json '.' "$1" | jq -r '(.labels // []) | join(",")'
}

template_name() {
	yq -o=json '.' "$1" | jq -r '.name // ""'
}

slugify() {
	printf '%s' "$1" |
		tr '[:upper:]' '[:lower:]' |
		sed -E 's/[^a-z0-9]+/-/g; s/^-+//; s/-+$//' |
		cut -c1-40 |
		sed -E 's/-+$//'
}

# gum exits non-zero when the prompt is cancelled; treat that as abort.
prompt_input() {
	local value
	value=$(gum input --header "$1" --placeholder "$2" --value "$3" --width 0) ||
		fail "cancelled"
	printf '%s' "$value"
}

prompt_choose() {
	local header=$1
	shift
	local value
	value=$(gum choose --header "$header" "$@") || fail "cancelled"
	printf '%s' "$value"
}

# Editors that return immediately unless told to block.
editor_command() {
	local cmd=$1
	case ${cmd%% *} in
	code | code-insiders | codium | cursor | windsurf)
		printf '%s --wait' "$cmd"
		;;
	*)
		printf '%s' "$cmd"
		;;
	esac
}

main() {
	local issue_type="" branch="" editor_cmd="" assignee=$DEFAULT_ASSIGNEE
	local base="" premise="" checkout=1 dry_run=0 assume_yes=0

	while (($#)); do
		case $1 in
		-t | --type)
			issue_type=${2:?--type needs a value}
			shift 2
			;;
		-b | --branch)
			branch=${2:?--branch needs a value}
			shift 2
			;;
		-e | --editor)
			editor_cmd=${2:?--editor needs a value}
			shift 2
			;;
		-a | --assignee)
			assignee=${2:?--assignee needs a value}
			shift 2
			;;
		--base)
			base=${2:?--base needs a value}
			shift 2
			;;
		--no-checkout)
			checkout=0
			shift
			;;
		-y | --yes)
			assume_yes=1
			shift
			;;
		-n | --dry-run)
			dry_run=1
			shift
			;;
		-h | --help)
			usage
			return 0
			;;
		--)
			shift
			premise=${*:-}
			break
			;;
		-*)
			fail "unknown option: $1 (see --help)"
			;;
		*)
			premise=$1
			shift
			;;
		esac
	done

	require_tools

	local repo_root
	repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || fail "not inside a git repository"
	cd "$repo_root"

	if [[ -z $issue_type ]]; then
		[[ -t 0 ]] || fail "no tty; pass --type, --branch and a premise"
		issue_type=$(prompt_choose "Issue type" feature bug)
	fi

	local template=$TEMPLATE_DIR/$issue_type.yml
	[[ -f $template ]] || fail "no issue form at $template"

	if [[ -z $premise ]]; then
		[[ -t 0 ]] || fail "no tty; pass the premise as an argument"
		premise=$(prompt_input "Premise" "One line describing the issue" "")
	fi
	[[ -n $premise ]] || fail "premise cannot be empty"

	if [[ -z $branch ]]; then
		[[ -t 0 ]] || fail "no tty; pass --branch"
		local prefix=fix
		[[ $issue_type == feature ]] && prefix=feat
		branch=$(prompt_input "Branch" "branch name" "$prefix/$(slugify "$premise")")
	fi
	[[ -n $branch ]] || fail "branch cannot be empty"

	local repo
	repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner) || fail "cannot resolve the GitHub repository"
	if [[ -z $base ]]; then
		base=$(gh repo view --json defaultBranchRef --jq .defaultBranchRef.name) ||
			fail "cannot resolve the default branch"
	fi

	local labels body
	labels=$(template_labels "$template")
	body=$(render_body "$template")
	[[ -n $body ]] || fail "rendered an empty body from $template"

	local editor_full
	editor_full=$(editor_command "${editor_cmd:-${GH_EDITOR:-$DEFAULT_EDITOR_CMD}}")

	gum style --border rounded --padding "0 1" --border-foreground 212 \
		"$( ((dry_run)) && printf 'DRY RUN: ' || printf '')$repo" \
		"form:     $(template_name "$template") ($template)" \
		"title:    $premise" \
		"labels:   ${labels:-none}" \
		"assignee: $assignee" \
		"branch:   $branch (from $base)" \
		"editor:   $editor_full"

	local -a create_args=(
		--title "$premise"
		--assignee "$assignee"
	)
	[[ -n $labels ]] && create_args+=(--label "$labels")

	local -a develop_cmd=(gh issue develop '<issue-number>' --name "$branch" --base "$base")
	((checkout)) && develop_cmd+=(--checkout)

	if ((dry_run)); then
		printf '\n--- rendered body ---\n%s\n--- end body ---\n\n' "$body"
		printf 'would open: %s <rendered form>\n' "$editor_full"
		printf 'would run:  gh issue create %s --body-file <edited form>\n' "${create_args[*]@Q}"
		printf 'would run:  %s\n' "${develop_cmd[*]@Q}"
		return 0
	fi

	if ((!assume_yes)); then
		[[ -t 0 ]] || fail "no tty for confirmation; pass --yes"
		gum confirm "Create this issue?" || fail "cancelled"
	fi

	body_file=$(mktemp -t new-issue.XXXXXX.md)
	printf '%s\n' "$body" >"$body_file"

	local -a editor=()
	read -r -a editor <<<"$editor_full"
	have "${editor[0]}" || fail "editor not found: ${editor[0]}"
	"${editor[@]}" "$body_file" || fail "editor exited non-zero; nothing created"

	local edited
	edited=$(<"$body_file")
	[[ -n ${edited//[[:space:]]/} ]] || fail "body is empty; nothing created"
	if [[ $edited == "$body" ]] && ((!assume_yes)); then
		gum confirm "The form is still unfilled. Create it anyway?" || fail "cancelled"
	fi

	local url
	url=$(gh issue create "${create_args[@]}" --body-file "$body_file") ||
		fail "gh issue create failed"

	local number=${url##*/}
	[[ $number =~ ^[0-9]+$ ]] || fail "could not read an issue number from: $url"

	local -a develop=(gh issue develop "$number" --name "$branch" --base "$base")
	((checkout)) && develop+=(--checkout)
	"${develop[@]}" || fail "issue $number was created, but creating branch '$branch' failed"

	gum style --border rounded --padding "0 1" --border-foreground 42 \
		"issue:  $url" \
		"branch: $branch" \
		"next:   gh pr create --fill --draft"
}

main "$@"
