// Package sql provides functions to trace the database/sql package (https://golang.org/pkg/database/sql).
// It will automatically augment operations such as connections, statements and transactions with tracing.
//
// We start by telling the package which driver we will be using. For example, if we are using "github.com/lib/pq",
// we would do as follows:
//
// 	sqltrace.Register("pq", pq.Driver{})
//	db, err := sqltrace.Open("pq", "postgres://pqgotest:password@localhost...")
//
// The rest of our application would continue as usual, but with tracing enabled.
//
package sql

import (
	"database/sql"
	"database/sql/driver"
	"errors"

	"github.com/DataDog/dd-trace-go/tracer"
)

// Register tells the sql integration package about the driver that we will be tracing. It must
// be called before Open, if that connection is to be traced. It uses the driverName suffixed
// with ".db" as the default service name. To set a custom service name, use RegisterWithServiceName.
func Register(driverName string, driver driver.Driver) {
	serviceName := driverName + ".db"
	RegisterWithServiceName(serviceName, driverName, driver)
}

// RegisterWithServiceName performs the same operation as Register, but allows setting a custom service name.
func RegisterWithServiceName(serviceName, driverName string, driver driver.Driver) {
	if driver == nil {
		panic("sqltrace: Register driver is nil")
	}
	name := tracedDriverName(driverName)
	if driverExists(name) {
		// no problem, carry on
		return
	}
	// TODO(gbbr): Change this when switching to OpenTracing.
	t := tracer.DefaultTracer
	sql.Register(name, &tracedDriver{
		Driver:      driver,
		tracer:      t,
		driverName:  driverName,
		serviceName: serviceName,
	})
}

// errNotRegistered is returned when there is an attempt to open a database connection towards a driver
// that has not previously been registered using this package.
var errNotRegistered = errors.New("sqltrace: Register must be called before Open")

// Open returns connection to a DB using a the traced version of the given driver. In order for Open
// to work, the driver must first be registered using Register or RegisterWithServiceName. If this
// did not occur, Open will return an error.
func Open(driverName, dataSourceName string) (*sql.DB, error) {
	name := tracedDriverName(driverName)
	if !driverExists(name) {
		return nil, errNotRegistered
	}
	return sql.Open(name, dataSourceName)
}
