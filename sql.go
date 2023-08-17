// Package sql provides a javascript module for performing SQL actions against relational databases
package sql

import (
	dbsql "database/sql"
	"fmt"
	"reflect"

	pg "github.com/lib/pq"
  _ "github.com/ClickHouse/clickhouse-go/v2"
  "go.k6.io/k6/js/modules"
)

func init() {
	modules.Register("k6/x/sql", new(RootModule))
}

// RootModule is the global module object type. It is instantiated once per test
// run and will be used to create `k6/x/sql` module instances for each VU.
type RootModule struct{}

// SQL represents an instance of the SQL module for every VU.
type SQL struct {
	vu modules.VU
  Driver string
}

// Ensure the interfaces are implemented correctly.
var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &SQL{}
)

// NewModuleInstance implements the modules.Module interface to return
// a new instance for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &SQL{vu: vu}
}

// Exports implements the modules.Instance interface and returns the exports
// of the JS module.
func (sql *SQL) Exports() modules.Exports {
	return modules.Exports{Default: sql}
}

// KeyValue is a simple key-value pair.
type KeyValue map[string]interface{}

func contains(array []string, element string) bool {
	for _, item := range array {
		if item == element {
			return true
		}
	}
	return false
}

// Open establishes a connection to the specified database type using
// the provided connection string.
func (sql *SQL) Open(database string, connectionString string) (*dbsql.DB, error) {
	supportedDatabases := []string{"clickhouse", "postgres"}
	if !contains(supportedDatabases, database) {
		return nil, fmt.Errorf("database %s is not supported", database)
	}
  sql.Driver = database
	db, err := dbsql.Open(database, connectionString)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (sql *SQL) QueryArray(db *dbsql.DB, query string, args ...interface{}) ([]KeyValue, error) {
  var us []interface{}
  if sql.Driver == "postgres" {
    us = make([]interface{}, len(args))
    for i, v := range args {
      rt := reflect.TypeOf(v)
      switch rt.Kind() {
      case reflect.Array, reflect.Slice:
        us[i] = pg.Array(v)
      default:
        us[i] = v
      }
    }
  } else {
    us = args
  }
  results, err := sql.Query(db, query, us...)
	if err != nil {
		return nil, err
	}
	return results, nil
}

// Query executes the provided query string against the database, while
// providing results as a slice of KeyValue instance(s) if available.
func (*SQL) Query(db *dbsql.DB, query string, args ...interface{}) ([]KeyValue, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = rows.Close()
	}()
	if rows.Err() != nil {
		return nil, rows.Err()
	}

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	result := make([]KeyValue, 0)

	for rows.Next() {
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		err = rows.Scan(valuePtrs...)
		if err != nil {
			return nil, err
		}

		data := make(KeyValue, len(cols))
		for i, colName := range cols {
			data[colName] = *valuePtrs[i].(*interface{}) //nolint:forcetypeassert
		}
		result = append(result, data)
	}

	return result, nil
}
