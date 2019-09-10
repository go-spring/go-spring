/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package SpringMysql

import (
	"database/sql"
	"reflect"
)

type MysqlTemplateImpl struct {
	DB *sql.DB
}

func (impl *MysqlTemplateImpl) Query(r interface{}, query string, args ...interface{}) error {

	stmt, err := impl.DB.Prepare(query)
	if err != nil {
		return err
	}

	defer stmt.Close()

	rows, err := stmt.Query(args...)
	if err != nil {
		return err
	}

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	var v reflect.Value
	var et reflect.Type

	count := 0

	for rows.Next() {

		if count == 0 {
			t := reflect.TypeOf(r).Elem()
			v = reflect.New(t).Elem()
			et = t.Elem()
		}

		count++

		ev := reflect.New(et).Elem()
		fm := make(map[string]interface{})

		for i := 0; i < et.NumField(); i++ {
			s := et.Field(i).Tag.Get("column")
			fm[s] = ev.Field(i).Addr().Interface()
		}

		dest := make([]interface{}, 0)

		for _, column := range columns {
			dest = append(dest, fm[column])
		}

		if err := rows.Scan(dest...); err != nil {
			return err
		}

		v = reflect.Append(v, ev)
		break
	}

	if count > 0 {
		reflect.ValueOf(r).Elem().Set(v)
	}

	return nil
}

func (impl *MysqlTemplateImpl) QueryRow(res interface{}, query string, args ...interface{}) error {

	stmt, err := impl.DB.Prepare(query)
	if err != nil {
		return err
	}

	defer stmt.Close()

	rows, err := stmt.Query(args...)
	if err != nil {
		return err
	}

	if rows.Next() {

		columns, err := rows.Columns()
		if err != nil {
			return err
		}

		t := reflect.TypeOf(res).Elem()
		v := reflect.ValueOf(res).Elem()

		fm := make(map[string]interface{})

		for i := 0; i < t.NumField(); i++ {
			s := t.Field(i).Tag.Get("column")
			fm[s] = v.Field(i).Addr().Interface()
		}

		dest := make([]interface{}, 0)

		for _, column := range columns {
			dest = append(dest, fm[column])
		}

		if err := rows.Scan(dest...); err != nil {
			return err
		}
	}

	return nil
}
