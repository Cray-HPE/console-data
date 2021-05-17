/*
 * Copyright 2021 Hewlett Packard Enterprise Development LP
 *
 * Permission is hereby granted, free of charge, to any person obtaining a
 * copy of this software and associated documentation files (the "Software"),
 * to deal in the Software without restriction, including without limitation
 * the rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 * 
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL
 * THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
 * OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
 * ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
 * OTHER DEALINGS IN THE SOFTWARE.
 * 
 * (MIT License)
 */

// This file contains the database implementations for the API.

package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/lib/pq" //needed for DB stuff
	"log"
	"sync"
)

// DB - the Database connection
var DB *sql.DB

// Prevent synchronous access by multiple concurrent requests where needed.
var mu sync.Mutex

// Initialize the DB connection.
func initDBConn() {

	dbUserName := getEnv("POSTGRES_USER", "console")
	dbName := getEnv("POSTGRES_DB", "service_db")
	dbHostName := getEnv("POSTGRES_HOST", "console-data-cray-console-data-postgres")
	dbPort := getEnv("POSTGRES_PORT", "5432")
	dbPasswd := getEnv("POSTGRES_PASSWD", "")

	connStr := fmt.Sprintf("sslmode=disable user=%s dbname=%s host=%s port=%s", dbUserName, dbName,
		dbHostName, dbPort)

	log.Printf("Attempt to open DB conn as: %s", connStr)
	connStr += " password=" + dbPasswd
	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Panicf("Unable to open DB connection: %s", err)
	}
	log.Printf("Opened DB conn")
}

// Prepare the DB if needed.
func prepareDB() (err error) {

	create_table := `
	CREATE TABLE IF NOT EXISTS ownership (
		node_name VARCHAR( 50 )  PRIMARY KEY NOT NULL CHECK (node_name <> ''),
        node_bmc_name VARCHAR( 50 )  NOT NULL CHECK (node_bmc_name <> ''),
        node_bmc_fqdn VARCHAR( 50 )  NOT NULL CHECK (node_bmc_fqdn <> ''),
        node_class VARCHAR( 50 )  NOT NULL CHECK (node_class <> ''),
        node_nid_number INTEGER  NOT NULL CHECK (node_nid_number <> 0),
        node_role VARCHAR( 50 )  NOT NULL CHECK (node_role <> ''),
		console_pod_id VARCHAR( 50 ),
		last_updated TIMESTAMP,
		heartbeat TIMESTAMP
	);`

	if _, err := DB.Exec(create_table); err != nil {
		return err
	}
	return nil
}

// dbConsolePodAcquireNodes will attempt to acquire the numbers of nodes requested by type.
// All acquired nodes will be added to the NodeConsoleInfo array.  Any error(s) will be returned.
func dbConsolePodAcquireNodes(pod_id string, numMtn, numRvr int) (rowsAffected int64, acquired []NodeConsoleInfo, err error) {

	// Exit quickly when no nodes were requested.
	if numMtn < 1 && numRvr < 1 {
		log.Printf("dbConsolePodAcquireNodes: the requested number of Mtn and Rvr was zero.  Returning.")
		return 0, []NodeConsoleInfo{}, nil
	}

	mu.Lock()
	defer mu.Unlock()
	var nodes string
	var errList []string
	acquired = []NodeConsoleInfo{}

	sqlStmt := `
		select node_name, node_bmc_name, node_bmc_fqdn, node_class, node_nid_number, node_role
		from ownership
		where node_class=$1 and console_pod_id is NULL
		limit $2
	`
	var i int = 0
	if numMtn > 0 {
		rows, err := DB.Query(sqlStmt, "Mountain", numMtn)
		defer rows.Close()
		if err != nil {
			errMsg := fmt.Sprintf("WARN: dbConsolePodAcquireNodes: There is a SELECT error: %s", err)
			log.Printf(errMsg)
			errList = append(errList, errMsg)
		}
		if rows != nil {
			for rows.Next() {
				var nci NodeConsoleInfo
				err := rows.Scan(&nci.NodeName,
					&nci.BmcName,
					&nci.BmcFqdn,
					&nci.Class,
					&nci.NID,
					&nci.Role)
				if err != nil {
					errList = append(errList, fmt.Sprintf("WARN: dbConsolePodAcquireNodes: Error scanning row: %s", err))
					continue // Try next record.
				}
				acquired = append(acquired, nci)
				if i > 0 {
					nodes += fmt.Sprintf(",'%s' ", nci.NodeName)
				} else {
					nodes += fmt.Sprintf("'%s' ", nci.NodeName)
				}
				i++
			}
		}
	}

	if numRvr > 0 {
		rows, err := DB.Query(sqlStmt, "River", numRvr)
		defer rows.Close()
		if err != nil {
			errMsg := fmt.Sprintf("WARN: dbClearStaleNodes: There is a SELECT error: %s", err)
			log.Printf(errMsg)
			errList = append(errList, errMsg)
		}
		if rows != nil {
			for rows.Next() {
				var nci NodeConsoleInfo
				err := rows.Scan(&nci.NodeName,
					&nci.BmcName,
					&nci.BmcFqdn,
					&nci.Class,
					&nci.NID,
					&nci.Role)
				if err != nil {
					errList = append(errList, fmt.Sprintf("WARN: dbConsolePodAcquireNodes: Error scanning row: %s", err))
					continue // Try next record.
				}
				acquired = append(acquired, nci)
				if i > 0 {
					nodes += fmt.Sprintf(",'%s' ", nci.NodeName)
				} else {
					nodes += fmt.Sprintf("'%s' ", nci.NodeName)
				}
				i++
			}
		}
	}

	if len(nodes) > 0 {
		sqlStmt = `
			update ownership set console_pod_id = '%s', heartbeat=now()
			where node_name in (%s)
		`
		debugLog.Println(fmt.Sprintf("pod_id=%s nodes=%s", pod_id, nodes))
		sqlStmt = fmt.Sprintf(sqlStmt, pod_id, nodes)
		debugLog.Println(fmt.Sprintf("dbConsolePodAcquireNodes running: %s", sqlStmt))
		result, err := DB.Exec(sqlStmt)
		rowsAffected = 0
		if err != nil {
			errMsg := fmt.Sprintf("WARN: dbConsolePodAcquireNodes: There is an UPDATE error: %s", err)
			log.Printf(errMsg)
			errList = append(errList, errMsg)
		}
		if result != nil {
			// On an update operation RowsAffected will be the count acually updated.
			rowsAffected, _ = result.RowsAffected()
			debugLog.Println(fmt.Sprintf("result.RowsAffected %d", rowsAffected))
		}
	}

	if len(errList) > 0 {
		var errStr string
		for _, e := range errList {
			errStr += fmt.Sprintf("%s\n", e)
		}
		return rowsAffected, acquired, errors.New(errStr)
	} else {
		return rowsAffected, acquired, nil
	}

}

// dbUpdateNodes will ensure that the list of node metadata exists in the database.
// Any error(s) will be returned.
func dbUpdateNodes(ncis *[]NodeConsoleInfo) (rowsInserted int64, err error) {

	// Insert each node.  Duplicates will be ignored.
	// Any errors will be logged and returned.
	// This first cut is non-transactional meaning that any
	// inserts that can be completed will immediately complete.
	var errList []string
	rowsInserted = 0
	sql := `
        insert into ownership (node_name,
          node_bmc_name,
          node_bmc_fqdn,
          node_class,
          node_nid_number,
          node_role,
          console_pod_id,
          last_updated,
	      heartbeat)
        values
          ($1,
          $2,
          $3,
          $4,
          $5,
          $6,
          NULL,
          now(),
	      NULL)
        on conflict (node_name) do nothing
    `
	for _, nci := range *ncis {
		result, err := DB.Exec(sql,
			nci.NodeName,
			nci.BmcName,
			nci.BmcFqdn,
			nci.Class,
			nci.NID,
			nci.Role)
		if err != nil {
			errMsg := fmt.Sprintf("WARN: dbUpdateNodes: There is an INSERT error on node %s: %s", nci.NodeName, err)
			log.Printf(errMsg)
			errList = append(errList, errMsg)
		}
		if result != nil {
			// On an insert operation RowsAffected will be the count acually inserted.
			// This will be 1 for new records and 0 for a duplicate which is ignored or
			// in the case of a check constraint violation.
			i64, _ := result.RowsAffected()
			debugLog.Println(fmt.Sprintf("result.RowsAffected %d", i64))
			rowsInserted += i64
		}
	}
	if len(errList) > 0 {
		var errStr string
		for _, e := range errList {
			errStr += fmt.Sprintf("%s\n", e)
		}
		return rowsInserted, errors.New(errStr)
	} else {
		return rowsInserted, nil
	}
}

// dbClearStaleNodes will clear the console pod id for any node that has a timestamp
// older than the duration provided here. Any error(s) will be returned.
func dbClearStaleNodes(duration int) (rowsAffected int64, err error) {

	mu.Lock()
	defer mu.Unlock()
	sqlStmt := `
		update ownership set console_pod_id=NULL, heartbeat=NULL
		where heartbeat < now()::timestamp - INTERVAL '%d minute'
	`
	sqlStmt = fmt.Sprintf(sqlStmt, duration) // DB.exec will not substitute
	result, err := DB.Exec(sqlStmt)
	rowsAffected = 0
	if err != nil {
		errMsg := fmt.Sprintf("WARN: dbClearStaleNodes: There is an UPDATE error: %s", err)
		log.Printf(errMsg)
		err = errors.New(errMsg)
	}
	if result != nil {
		// On an update operation RowsAffected will be the count acually updated.
		rowsAffected, _ = result.RowsAffected()
		debugLog.Println(fmt.Sprintf("result.RowsAffected %d", rowsAffected))
	}

	return rowsAffected, err
}

// dbFindConsolePodForNode will find the node console assigned to the given node.
// Any error(s) will be returned.
func dbFindConsolePodForNode(nci *NodeConsoleInfo) (err error) {

	// Look for the node and if found set *nci.NodeConsoleName = console_pod_id
	// Return any error found.
	sqlStmt := `
        select console_pod_id from ownership where node_name=$1
    `
	if nci == nil || nci.NodeName == "" {
		return errors.New("Nil or empty NodeName.")
	}
	var s sql.NullString
	row := DB.QueryRow(sqlStmt, nci.NodeName)
	err = row.Scan(&s)
	switch err {
	case sql.ErrNoRows:
		// We did not find the node.
		// Signal that we did not find a console pod.
		nci.NodeConsoleName = ""
		log.Printf("Unable to find node %s", nci.NodeName)
		return nil
	case nil:
		if s.Valid {
			// We found the console pod.  Set it here.
			nci.NodeConsoleName = s.String
			log.Printf("Found console_pod_id %s for node %s",
				nci.NodeConsoleName, nci.NodeName)
		} else {
			// This is a NULL value.
			// Signal that we did not find a console pod.
			nci.NodeConsoleName = ""
		}
		return nil
	default:
		// Signal that we did not find a console pod.
		nci.NodeConsoleName = ""
		// Return the error.
		log.Printf("dbFindConsolePodForNode had an error: %s", err)
		return err
	}

	return nil
}

// dbConsolePodHeartbeat will update the heartbeat for all nodes assigned
// to this console pod and remove the node from the ncis list.
// Any nodes not assigned to the console pod will remain in ncis.
// Any error(s) will be returned.
func dbConsolePodHeartbeat(pod_id string, ncis *[]NodeConsoleInfo) (rowsAffected int64, notUpdated []NodeConsoleInfo, err error) {

	// Update all pods found by name and console pod ID.
	// All errors are returned.
	// This first cut is non-transactional meaning that any
	// updates that can be completed will immediately complete.
	mu.Lock()
	defer mu.Unlock()
	var errList []string
	rowsAffected = 0
	notUpdated = []NodeConsoleInfo{}

	sqlStmt := `
		update ownership set heartbeat=now()
		where node_name = $1 and console_pod_id = $2
	`
	for _, nci := range *ncis {
		result, err := DB.Exec(sqlStmt, nci.NodeName, pod_id)
		if err != nil {
			errMsg := fmt.Sprintf("WARN: dbConsolePodHeartbeat: There is an UPDATE error: %s", err)
			log.Printf(errMsg)
			errList = append(errList, errMsg)
		}
		if result != nil {
			// On an update operation RowsAffected will be the count acually updated.
			ra, _ := result.RowsAffected()
			debugLog.Println(fmt.Sprintf("result.RowsAffected %d", ra))
			if ra == 0 {
				// This node was not assigned to the pod.
				notUpdated = append(notUpdated, nci)
			} else {
				// Add the update count to the total.
				rowsAffected += ra
			}
		}

	}
	// Let the caller see the list that was not updated (if any).
	for _, nci := range notUpdated {
		log.Printf("nci not updaed: %s", nci.NodeName)
	}

	if len(errList) > 0 {
		var errStr string
		for _, e := range errList {
			errStr += fmt.Sprintf("%s\n", e)
		}
		return rowsAffected, notUpdated, errors.New(errStr)
	} else {
		return rowsAffected, notUpdated, nil
	}
}

/*
dbConsolePodRelease will remove the console pod from all nodes in the list.
  takes []NodeConsoleInfo - pod no longer monitors these nodes, free for acquisition
*/
func dbConsolePodRelease(pod_id string, ncis *[]NodeConsoleInfo) (rowsAffected int64, err error) {
	// exit fast
	if pod_id == "" || ncis == nil || len(*ncis) == 0 {
		return 0, nil
	}

	var nodes string
	i := 0
	for _, nci := range *ncis {
		if i > 0 {
			nodes += fmt.Sprintf(",'%s' ", nci.NodeName)
		} else {
			nodes += fmt.Sprintf("'%s' ", nci.NodeName)
		}
		i++
	}

	sqlStmt := `
		update ownership set console_pod_id=NULL, heartbeat=NULL
		where console_pod_id = '%s'
		and node_name in (%s)
	`
	sqlStmt = fmt.Sprintf(sqlStmt, pod_id, nodes)
	mu.Lock()
	defer mu.Unlock()
	result, err := DB.Exec(sqlStmt)
	rowsAffected = 0
	if err != nil {
		errMsg := fmt.Sprintf("WARN: dbConsolePodRelease: There is an UPDATE error: %s", err)
		log.Printf(errMsg)
		err = errors.New(errMsg)
	}
	if result != nil {
		// On an update operation RowsAffected will be the count acually updated.
		rowsAffected, _ = result.RowsAffected()
		debugLog.Println(fmt.Sprintf("result.RowsAffected %d", rowsAffected))
	}

	return rowsAffected, err

}

/*
dbDeleteNodes will remove nodes from the provided list from the inventory.
  takes []NodeConsoleInfo - these nodes are no longer in the system at all
*/
func dbDeleteNodes(ncis *[]NodeConsoleInfo) (rowsAffected int64, err error) {
	// exit fast
	if ncis == nil || len(*ncis) == 0 {
		return 0, nil
	}

	var nodes string
	i := 0
	for _, nci := range *ncis {
		if i > 0 {
			nodes += fmt.Sprintf(",'%s' ", nci.NodeName)
		} else {
			nodes += fmt.Sprintf("'%s' ", nci.NodeName)
		}
		i++
	}

	sqlStmt := `
		delete from ownership
		where node_name in (%s)
	`
	sqlStmt = fmt.Sprintf(sqlStmt, nodes)
	mu.Lock()
	defer mu.Unlock()
	result, err := DB.Exec(sqlStmt)
	rowsAffected = 0
	if err != nil {
		errMsg := fmt.Sprintf("WARN: dbDeleteNodes: There is a DELETE error: %s", err)
		log.Printf(errMsg)
		err = errors.New(errMsg)
	}
	if result != nil {
		// On an update operation RowsAffected will be the count acually updated.
		rowsAffected, _ = result.RowsAffected()
		debugLog.Println(fmt.Sprintf("result.RowsAffected %d", rowsAffected))
	}
	return rowsAffected, err

}
