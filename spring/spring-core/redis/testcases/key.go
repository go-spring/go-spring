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

//DEL
//redis> SET key1 "Hello"
//"OK"
//redis> SET key2 "World"
//"OK"
//redis> DEL key1 key2 key3
//(integer) 2
//redis>

//DUMP
//redis> SET mykey 10
//"OK"
//redis> DUMP mykey
//"\u0000\xC0\n\t\u0000\xBEm\u0006\x89Z(\u0000\n"
//redis>

//EXISTS
//redis> SET key1 "Hello"
//"OK"
//redis> EXISTS key1
//(integer) 1
//redis> EXISTS nosuchkey
//(integer) 0
//redis> SET key2 "World"
//"OK"
//redis> EXISTS key1 key2 nosuchkey
//(integer) 2
//redis>

//EXPIRE
//redis> SET mykey "Hello"
//"OK"
//redis> EXPIRE mykey 10
//(integer) 1
//redis> TTL mykey
//(integer) 10
//redis> SET mykey "Hello World"
//"OK"
//redis> TTL mykey
//(integer) -1
//redis> EXPIRE mykey 10 XX
//ERR ERR wrong number of arguments for 'expire' command
//redis> TTL mykey
//(integer) -1
//redis> EXPIRE mykey 10 NX
//ERR ERR wrong number of arguments for 'expire' command
//redis> TTL mykey
//(integer) -1
//redis>

//EXPIREAT
//redis> SET mykey "Hello"
//"OK"
//redis> EXISTS mykey
//(integer) 1
//redis> EXPIREAT mykey 1293840000
//(integer) 1
//redis> EXISTS mykey
//(integer) 0
//redis>

//KEYS
//redis> MSET firstname Jack lastname Stuntman age 35
//"OK"
//redis> KEYS *name*
//1) "firstname"
//2) "lastname"
//redis> KEYS a??
//1) "age"
//redis> KEYS *
//1) "firstname"
//2) "age"
//3) "lastname"
//redis>

//PERSIST
//redis> SET mykey "Hello"
//"OK"
//redis> EXPIRE mykey 10
//(integer) 1
//redis> TTL mykey
//(integer) 10
//redis> PERSIST mykey
//(integer) 1
//redis> TTL mykey
//(integer) -1
//redis>

//PEXPIRE
//redis> SET mykey "Hello"
//"OK"
//redis> PEXPIRE mykey 1500
//(integer) 1
//redis> TTL mykey
//(integer) 1
//redis> PTTL mykey
//(integer) 1499
//redis> PEXPIRE mykey 1000 XX
//ERR ERR wrong number of arguments for 'pexpire' command
//redis> TTL mykey
//(integer) 1
//redis> PEXPIRE mykey 1000 NX
//ERR ERR wrong number of arguments for 'pexpire' command
//redis> TTL mykey
//(integer) 1
//redis>

//PEXPIREAT
//redis> SET mykey "Hello"
//"OK"
//redis> PEXPIREAT mykey 1555555555005
//(integer) 1
//redis> TTL mykey
//(integer) -2
//redis> PTTL mykey
//(integer) -2
//redis>

//PTTL
//redis> SET mykey "Hello"
//"OK"
//redis> EXPIRE mykey 1
//(integer) 1
//redis> PTTL mykey
//(integer) 999
//redis>

//RENAME
//redis> SET mykey "Hello"
//"OK"
//redis> RENAME mykey myotherkey
//"OK"
//redis> GET myotherkey
//"Hello"
//redis>

//RENAMENX
//redis> SET mykey "Hello"
//"OK"
//redis> SET myotherkey "World"
//"OK"
//redis> RENAMENX mykey myotherkey
//(integer) 0
//redis> GET myotherkey
//"World"
//redis>

//RESTORE
//redis> DEL mykey
//0
//redis> RESTORE mykey 0 "\n\x17\x17\x00\x00\x00\x12\x00\x00\x00\x03\x00\
//                        x00\xc0\x01\x00\x04\xc0\x02\x00\x04\xc0\x03\x00\
//                        xff\x04\x00u#<\xc0;.\xe9\xdd"
//OK
//redis> TYPE mykey
//list
//redis> LRANGE mykey 0 -1
//1) "1"
//2) "2"
//3) "3"

//TOUCH
//redis> SET key1 "Hello"
//"OK"
//redis> SET key2 "World"
//"OK"
//redis> TOUCH key1 key2
//(integer) 2
//redis>

//TTL
//redis> SET mykey "Hello"
//"OK"
//redis> EXPIRE mykey 10
//(integer) 1
//redis> TTL mykey
//(integer) 10
//redis>

//TYPE
//redis> SET key1 "value"
//"OK"
//redis> LPUSH key2 "value"
//(integer) 1
//redis> SADD key3 "value"
//(integer) 1
//redis> TYPE key1
//"string"
//redis> TYPE key2
//"list"
//redis> TYPE key3
//"set"
//redis>
