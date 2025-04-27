#!/bin/bash

set -e
set -u

function create_database() {
	local database=$1
	echo "Creating database '$database'"
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" <<-EOSQL
	    CREATE DATABASE $database;
	    GRANT ALL PRIVILEGES ON DATABASE $database TO $POSTGRES_USER;
EOSQL
}

function apply_migrations() {
	local database=$1
	local service=$2
	echo "Applying migrations for '$database' from '$service'"
	for file in /migrations/$service/*.up.sql; do
		echo "Applying $file to $database"
		psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" -d "$database" -f "$file"
	done
}

if [ -n "$POSTGRES_MULTIPLE_DATABASES" ]; then
	echo "Multiple database creation requested: $POSTGRES_MULTIPLE_DATABASES"
	for db in $(echo $POSTGRES_MULTIPLE_DATABASES | tr ',' ' '); do
		create_database $db
	done
	echo "Creating orders database..."
	apply_migrations "orders" "order_service"
	echo "Creating billing database..."
	apply_migrations "billing" "billing_service"
	echo "Creating notifications database..."
	apply_migrations "notifications" "notification_service"
fi