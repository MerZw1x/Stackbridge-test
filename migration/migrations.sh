#!/bin/sh

if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo -n "Usage: sh migration/migratios.sh [OPTIONS]
OPTIONS:
    --help,   -h         - show this text and exit
    --new,    -n NAME    - generate new migration file
                             with given name
    --status, -s         - get migration status
    --up,     -u VERSION - apply all available migration.
                             If a version is given,
                             migrate up to a specific version
    --down,   -d VERSION - roll back a single migration
                             from the current version.
                             If a version is given,
                             roll back migration
                             to a specific version
"
    exit
elif [ "$1" = "--new" ] || [ "$1" = "-n" ]; then
    if [ -n "$2" ]; then
        goose -dir migration create $2 sql
        exit
    else
        echo "ERROR: forgot to add name"
        exit
    fi

elif [ "$1" = "--status" ] || [ "$1" = "-s" ]; then
    action=status
    
elif [ "$1" = "--up" ] || [ "$1" = "-u" ]; then
    action=up
    if [ -n "$2" ]; then
        action=up-to
        action_arg="$2"
    fi

elif [ "$1" = "--down" ] || [ "$1" = "-d" ]; then
    action=down
    if [ -n "$2" ]; then
        action=down-to
        action_arg="$2"
    fi

else
    echo "Try: sh migration/migration.sh --help"
    exit
fi

DATABASE_USER=$(grep -E "^DATABASE_USER=" .env | cut -d'=' -f2- | tr -d '\n\r')
DATABASE_PASSWORD=$(grep -E "^DATABASE_PASSWORD=" .env | cut -d'=' -f2- | tr -d '\n\r')
DATABASE_NAME=$(grep -E "^DATABASE_NAME=" .env | cut -d'=' -f2- | tr -d '\n\r')
DATABASE_PORT=$(grep -E "^DATABASE_PORT=" .env | cut -d'=' -f2- | tr -d '\n\r')

MIGRATION_HOST="localhost"

DSN="host=$MIGRATION_HOST port=$DATABASE_PORT user=$DATABASE_USER password=$DATABASE_PASSWORD dbname=$DATABASE_NAME sslmode=disable"

echo "Connecting to: host=$MIGRATION_HOST port=$DATABASE_PORT dbname=$DATABASE_NAME"

goose -dir migration postgres "$DSN" "$action" $action_arg