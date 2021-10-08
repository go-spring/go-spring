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

package testcases

//BITCOUNT
//redis> SET mykey "foobar"
//"OK"
//redis> BITCOUNT mykey
//(integer) 26
//redis> BITCOUNT mykey 0 0
//(integer) 4
//redis> BITCOUNT mykey 1 1
//(integer) 6
//redis>

//BITOP
//redis> SET key1 "foobar"
//"OK"
//redis> SET key2 "abcdef"
//"OK"
//redis> BITOP AND dest key1 key2
//(integer) 6
//redis> GET dest
//"`bc`ab"
//redis>

//BITPOS
//redis> SET mykey "\xff\xf0\x00"
//"OK"
//redis> BITPOS mykey 0
//(integer) 12
//redis> SET mykey "\x00\xff\xf0"
//"OK"
//redis> BITPOS mykey 1 0
//(integer) 8
//redis> BITPOS mykey 1 2
//(integer) 16
//redis> set mykey "\x00\x00\x00"
//"OK"
//redis> BITPOS mykey 1
//(integer) -1
//redis>

//GETBIT
//redis> SETBIT mykey 7 1
//(integer) 0
//redis> GETBIT mykey 0
//(integer) 0
//redis> GETBIT mykey 7
//(integer) 1
//redis> GETBIT mykey 100
//(integer) 0
//redis>

//SETBIT
//redis> SETBIT mykey 7 1
//(integer) 0
//redis> SETBIT mykey 7 0
//(integer) 1
//redis> GET mykey
//"\u0000"
//redis>
