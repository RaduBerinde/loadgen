// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License. See the AUTHORS file
// for names of contributors.
//
// Author: Arjun Narayan
//
// The TPC-H loader loads data generated by the TPC DBGen utility (currently
// version 2.17.0) into Cockroach.

package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/cockroachdb/cockroach-go/crdb"
	"github.com/pkg/errors"
)

func doInserts(db *sql.DB, preamble string, inserts []string) error {
	return crdb.ExecuteTx(db, func(*sql.Tx) error {
		allInserts := strings.Join(inserts, ", ")
		_, inErr := db.Exec(fmt.Sprintf("%s%s", preamble, allInserts))
		return inErr
	})
}

func insertTableFromFile(db *sql.DB, filename string, tableType table) error {
	if *verbose {
		fmt.Printf("Inserting table from file: %s\n", filename)
	}
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Error encountered when closing file '%s'\n: %s", filename, err)
		}
	}()

	scanner := bufio.NewScanner(file)
	var numTotalInserts uint
	inserts := make([]string, 0, *insertsPerTransaction)

	var insertPreamble, insertValues string
	switch tableType {
	case nation:
		insertPreamble = `INSERT INTO nation (n_nationkey, n_name, n_regionkey, n_comment) VALUES`
		insertValues = `(%s, '%s', %s, '%s')`
	case region:
		insertPreamble = `INSERT INTO region (r_regionkey, r_name, r_comment) VALUES`
		insertValues = ` (%s, '%s', '%s')`
	case part:
		insertPreamble = `INSERT INTO part (p_partkey, p_name, p_mfgr, p_brand, p_type,
                                            p_size, p_container, p_retailprice, p_comment) VALUES`
		insertValues = ` (%s, '%s', '%s', '%s', '%s', %s, '%s', %s, '%s')`
	case supplier:
		insertPreamble = `INSERT INTO supplier (s_suppkey, s_name, s_address, s_nationkey,
                                                s_phone, s_acctbal, s_comment) VALUES`
		insertValues = ` (%s, '%s', '%s', %s, '%s', %s, '%s')`
	case partsupp:
		insertPreamble = `INSERT INTO partsupp (ps_partkey, ps_suppkey, ps_availqty,
                                                ps_supplycost, ps_comment) VALUES`
		insertValues = ` (%s, %s, %s, %s, '%s')`
	case customer:
		insertPreamble = `INSERT INTO customer (c_custkey, c_name, c_address, c_nationkey,
                                                c_phone, c_acctbal, c_mktsegment, c_comment) VALUES`
		insertValues = ` (%s, '%s', '%s', %s, '%s', %s, '%s', '%s')`
	case orders:
		insertPreamble = `INSERT INTO orders (o_orderkey, o_custkey, o_orderstatus, o_totalprice,
                                              o_orderdate, o_orderpriority, o_clerk,
                                              o_shippriority, o_comment) VALUES`
		insertValues = ` (%s, %s, '%s', %s, '%s', '%s', '%s', %s, '%s')`
	case lineitem:
		insertPreamble = `INSERT INTO lineitem
                    (l_orderkey, l_partkey, l_suppkey, l_linenumber,
                     l_quantity, l_extendedprice, l_discount, l_tax,
                     l_returnflag, l_linestatus, l_shipdate, l_commitdate,
                     l_receiptdate, l_shipinstruct, l_shipmode, l_comment) VALUES`
		insertValues = ` (%s, %s, %s, %s, %s, %s, %s, %s,
                      '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s')`
	default:
		return errors.Errorf("Unknown table type: %d", tableType)

	}

	for scanner.Scan() {
		line := scanner.Text()
		splits := strings.Split(line, "|")
		fields := make([]interface{}, len(splits)-1)
		// Ignore the last index since dbgen uses '|' as a terminator, not a separator.
		for i := 0; i < (len(splits) - 1); i++ {
			fields[i] = splits[i]
		}

		inserts = append(inserts, fmt.Sprintf(insertValues, fields...))
		numTotalInserts++

		if numTotalInserts%(*insertsPerTransaction) == 0 {
			if err := doInserts(db, insertPreamble, inserts); err != nil {
				return err
			}
			fmt.Printf("Inserts for table %2d:     %8d\n", tableType, numTotalInserts)
			inserts = inserts[:0]
		}
	}

	// Do any remaining inserts
	if len(inserts) > 0 {
		return doInserts(db, insertPreamble, inserts)
	}
	return nil
}
