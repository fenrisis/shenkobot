# Local convenience targets for shipping & operating shenkobot on Hetzner CX22.
# Override HOST per-call: `make deploy HOST=deploy@1.2.3.4`

HOST    ?= deploy@shenkobot.example.com
APP_DIR ?= /opt/shenkobot
SSH     ?= ssh -o StrictHostKeyChecking=accept-new
RSYNC   ?= rsync -avz --delete \
            --exclude=.git --exclude=.env --exclude=.claude \
            --exclude=.idea --exclude=.vscode --exclude=shenkobot \
            --exclude=backups --exclude=*.log --exclude=*.sql

.PHONY: help ship deploy build up down restart logs ps backup install-timer dump push-dump restore-dump

help:
	@echo "Targets:"
	@echo "  ship           rsync sources to $(HOST):$(APP_DIR) (no .env, no *.sql)"
	@echo "  deploy         ship + rebuild + up -d"
	@echo "  build          docker compose build on host"
	@echo "  up             docker compose up -d"
	@echo "  down           docker compose down"
	@echo "  restart        restart bot container only"
	@echo "  logs           tail -f compose logs"
	@echo "  ps             docker compose ps"
	@echo "  backup         run pg_dump on host"
	@echo "  install-timer  install systemd backup timer (run once)"
	@echo ""
	@echo "Lift-and-shift state from local → server:"
	@echo "  dump           locally: pg_dump --data-only ../dump.sql (excl. migrations)"
	@echo "  push-dump      scp ../dump.sql to $(HOST):$(APP_DIR)/"
	@echo "  restore-dump   on server: psql < dump.sql (after migrations are applied)"

ship:
	$(RSYNC) ../ $(HOST):$(APP_DIR)/

deploy: ship build up

build:
	$(SSH) $(HOST) 'cd $(APP_DIR) && docker compose build bot'

up:
	$(SSH) $(HOST) 'cd $(APP_DIR) && docker compose up -d'

down:
	$(SSH) $(HOST) 'cd $(APP_DIR) && docker compose down'

restart:
	$(SSH) $(HOST) 'cd $(APP_DIR) && docker compose restart bot'

logs:
	$(SSH) $(HOST) 'cd $(APP_DIR) && docker compose logs -f --tail=200'

ps:
	$(SSH) $(HOST) 'cd $(APP_DIR) && docker compose ps'

backup:
	$(SSH) $(HOST) 'cd $(APP_DIR) && bash deploy/backup.sh'

install-timer:
	$(SSH) $(HOST) ' \
		sudo cp $(APP_DIR)/deploy/systemd/shenkobot-backup.service /etc/systemd/system/ && \
		sudo cp $(APP_DIR)/deploy/systemd/shenkobot-backup.timer /etc/systemd/system/ && \
		sudo systemctl daemon-reload && \
		sudo systemctl enable --now shenkobot-backup.timer && \
		systemctl list-timers shenkobot-backup.timer'

# Capture local state (data only, no migrations table) into ../dump.sql.
dump:
	cd .. && docker compose exec -T postgres bash -c \
		'pg_dump -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" --data-only --disable-triggers --exclude-table-data=migrations' \
		> dump.sql
	@printf 'wrote %s (%d lines, %d bytes)\n' \
		"../dump.sql" "$$(wc -l < ../dump.sql)" "$$(wc -c < ../dump.sql)"

# Ship dump.sql to the server (separate from `ship` because *.sql is gitignored / rsync-excluded).
push-dump:
	scp ../dump.sql $(HOST):$(APP_DIR)/dump.sql

# Restore on server. Run AFTER `deploy` (so schema migrations are in place) and AFTER
# `push-dump`. Sources .env on the server to resolve $$DB_USER / $$DB_NAME.
restore-dump:
	$(SSH) $(HOST) 'cd $(APP_DIR) && set -a && . ./.env && set +a && \
		docker compose exec -T postgres psql -U "$$DB_USER" -d "$$DB_NAME" < dump.sql'
