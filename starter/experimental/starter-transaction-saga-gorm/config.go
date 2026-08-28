/*
 * Copyright 2025 The Go-Spring Authors.
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

package StarterTransactionSagaGorm

// gormConfig binds ${spring.transaction.saga.gorm}. It configures the durable,
// gorm-backed saga-log Store contributed by this starter. The prefix currently
// carries no keys of its own — the *gorm.DB is autowired from the container —
// but it is kept so future store options land under it without a breaking move.
type gormConfig struct{}
