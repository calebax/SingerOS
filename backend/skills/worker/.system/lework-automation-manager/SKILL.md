---
name: lework-automation-manager
description: Manage LeWork scheduled automations for the current Agent Run.
---

# LeWork Automation Manager

Use the typed `leros automation` CLI to query or change scheduled tasks. Do not
call an HTTP endpoint directly and do not construct internal `rrule` values.

## Operations

Use `--json` when parsing command output. Before updating, pausing, resuming,
or deleting an existing automation, use `ls` or `get` to confirm the ID and
current schedule. Never guess an automation ID.

```text
leros automation ls [--keyword <text>] [--status enabled|paused] [--mode calendar|interval] [--offset N] [--limit N]
leros automation get <automation_id>
leros automation create --name <name> --prompt <instruction> --status enabled|paused --mode calendar|interval ...
leros automation update <automation_id> [--name <name>] [--prompt <instruction>] [--status enabled|paused] ...
leros automation status <automation_id> enabled|paused
leros automation delete <automation_id>
```

Use `--user-id <user_id>` when the operation targets a specific user, and add
`--json` when machine-readable output is useful.

## Schedule examples

```bash
leros automation create --user-id 2008 --json \
  --name "每日汇报" --prompt "整理今天的项目进展" --status enabled \
  --mode calendar --preset daily --hour 18 --minute 0

leros automation create --user-id 2008 --json \
  --name "定期检查" --prompt "检查项目告警" --status enabled \
  --mode interval --interval-minutes 30 --anchor-at 09:00
```

Timezone is optional and defaults to `Asia/Shanghai`. The first version has
no alternate aliases, run-now command, or execution-history command. Delete
is immediate and has no confirmation flag. Report the target user ID,
automation ID, status, and effective schedule after each operation.
