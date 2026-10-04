package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// launchScript supplies project-local discovery without changing trust or inherited hooks.
func launchScript(root string, handler map[string]any) ([]byte, error) {
	arguments := []string{"codex", "--no-daemon", "-C", root}
	linked, err := linkedWorktree(root)
	if err != nil {
		return nil, err
	}
	if linked {
		handlers, err := installedSiblingHandlers(root, handler)
		if err != nil {
			return nil, err
		}
		groups := make([]any, 0, len(handlers))
		for _, sibling := range handlers {
			groups = append(groups, map[string]any{hooksField: []any{sibling}})
		}
		configuration, err := inlineHookValue(groups)
		if err != nil {
			return nil, fmt.Errorf("workflow launch handlers: %w", err)
		}
		for _, event := range workflowEvents {
			arguments = append(arguments, "-c", "hooks."+event+"="+configuration)
		}
	}
	quoted := make([]string, len(arguments))
	for index, argument := range arguments {
		quoted[index] = shellQuote(argument)
	}
	return []byte("#!/usr/bin/env bash\n# Start the installed workflow hooks in Codex; this does not grant trust.\n# Review the displayed hooks with /hooks, then start a fresh session using this launcher.\nexec " + strings.Join(quoted, " ") + "\n"), nil
}

func linkedWorktree(root string) (bool, error) {
	gitEntry, err := os.Stat(filepath.Join(root, ".git"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("workflow Git entry: %w", err)
	}
	if gitEntry.IsDir() {
		return false, nil
	}
	command := exec.CommandContext(context.Background(), "git", "-C", root, "rev-parse", "--path-format=absolute", "--git-dir", "--git-common-dir")
	output, err := command.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("verify workflow linked checkout: %w: %s", err, strings.TrimSpace(string(output)))
	}
	directories := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(directories) != 2 {
		return false, fmt.Errorf("verify workflow linked checkout: expected Git directory and common directory")
	}
	return filepath.Clean(directories[0]) != filepath.Clean(directories[1]), nil
}

func inlineHookValue(value any) (string, error) {
	switch value := value.(type) {
	case string:
		encoded, err := json.Marshal(value)
		if err != nil {
			return "", fmt.Errorf("encode hook string: %w", err)
		}
		return string(encoded), nil
	case bool:
		return strconv.FormatBool(value), nil
	case int:
		return strconv.Itoa(value), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64), nil
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		fields := make([]string, 0, len(keys))
		for _, key := range keys {
			encoded, err := inlineHookValue(value[key])
			if err != nil {
				return "", err
			}
			fields = append(fields, strconv.Quote(key)+"="+encoded)
		}
		return "{" + strings.Join(fields, ",") + "}", nil
	case []any:
		items := make([]string, len(value))
		for index, item := range value {
			encoded, err := inlineHookValue(item)
			if err != nil {
				return "", err
			}
			items[index] = encoded
		}
		return "[" + strings.Join(items, ",") + "]", nil
	default:
		return "", fmt.Errorf("unsupported hook field type %T", value)
	}
}

// installedSiblingHandlers snapshots only verified installed providers in this Git repository.
// Every linked launcher uses the same ordered groups; trust keys cannot collide across siblings.
func installedSiblingHandlers(root string, current map[string]any) ([]map[string]any, error) {
	command := exec.CommandContext(context.Background(), "git", "-C", root, "worktree", "list", "--porcelain", "-z")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list workflow sibling checkouts: %w", err)
	}
	providers := map[string]map[string]any{filepath.Clean(root): current}
	for _, field := range strings.Split(string(output), "\x00") {
		if !strings.HasPrefix(field, "worktree ") {
			continue
		}
		sibling := filepath.Clean(strings.TrimPrefix(field, "worktree "))
		if sibling == filepath.Clean(root) {
			continue
		}
		receipt := filepath.Join(sibling, workflowReceipt)
		_, err := os.Stat(receipt)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect sibling workflow receipt: %w", err)
		}
		status, err := NewInstaller(sibling, "", "").Status()
		if err != nil {
			return nil, fmt.Errorf("verify sibling workflow installation %s: %w", sibling, err)
		}
		if !status.Installed || (!status.Intact && !onlyEditableInstructionsChanged(sibling, status.Problems)) {
			return nil, fmt.Errorf("sibling workflow installation %s is not intact: %s", sibling, strings.Join(status.Problems, "; "))
		}
		content, err := os.ReadFile(receipt)
		if err != nil {
			return nil, fmt.Errorf("read sibling workflow handler: %w", err)
		}
		var metadata installation
		if err := json.Unmarshal(content, &metadata); err != nil {
			return nil, fmt.Errorf("decode sibling workflow handler: %w", err)
		}
		if metadata.Handler == nil {
			return nil, fmt.Errorf("sibling workflow handler absent: %s", sibling)
		}
		providers[sibling] = metadata.Handler
		if len(providers) > 64 {
			return nil, fmt.Errorf("workflow launcher inventory exceeds 64 installed providers")
		}
	}
	roots := make([]string, 0, len(providers))
	for sibling := range providers {
		roots = append(roots, sibling)
	}
	sort.Strings(roots)
	handlers := make([]map[string]any, 0, len(roots))
	for _, sibling := range roots {
		handlers = append(handlers, providers[sibling])
	}
	return handlers, nil
}

// onlyEditableInstructionsChanged permits customization without accepting altered runtime assets.
func onlyEditableInstructionsChanged(root string, problems []string) bool {
	path := filepath.Join(workflowDirectory, "instructions.md")
	if len(problems) != 1 || problems[0] != "Missing or changed asset: "+path {
		return false
	}
	info, err := os.Lstat(filepath.Join(root, path))
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	content, err := os.ReadFile(filepath.Join(root, path))
	return err == nil && strings.TrimSpace(string(content)) != ""
}
