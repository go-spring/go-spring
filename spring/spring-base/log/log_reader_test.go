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

package log_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/log"
)

func TestXMLReader(t *testing.T) {
	r := &log.XMLReader{}
	b := `<Configuration>
		<Appenders>
			<Console name="Console">
				<LevelFilter level="warn"/>
			</Console>
		</Appenders>
		<Loggers>
			<Logger name="spring/spring-base/log_test" level="debug">
				<AppenderRef ref="Console">
					<Filters>
						<LevelFilter level="info"/>
					</Filters>
				</AppenderRef>
			</Logger>
			<Root level="debug">
				<LevelFilter level="info"/>
				<AppenderRef ref="Console"/>
			</Root>
		</Loggers>
	</Configuration>`
	node, err := r.Read([]byte(b))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, node, &log.Node{
		Label:      "Configuration",
		Attributes: map[string]string{},
		Children: []*log.Node{
			{
				Label:      "Appenders",
				Attributes: map[string]string{},
				Children: []*log.Node{
					{
						Label: "Console",
						Attributes: map[string]string{
							"name": "Console",
						},
						Children: []*log.Node{
							{
								Label: "LevelFilter",
								Attributes: map[string]string{
									"level": "warn",
								},
							},
						},
					},
				},
			},
			{
				Label:      "Loggers",
				Attributes: map[string]string{},
				Children: []*log.Node{
					{
						Label: "Logger",
						Attributes: map[string]string{
							"name":  "spring/spring-base/log_test",
							"level": "debug",
						},
						Children: []*log.Node{
							{
								Label: "AppenderRef",
								Attributes: map[string]string{
									"ref": "Console",
								},
								Children: []*log.Node{
									{
										Label:      "Filters",
										Attributes: map[string]string{},
										Children: []*log.Node{
											{
												Label: "LevelFilter",
												Attributes: map[string]string{
													"level": "info",
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Label: "Root",
						Attributes: map[string]string{
							"level": "debug",
						},
						Children: []*log.Node{
							{
								Label: "LevelFilter",
								Attributes: map[string]string{
									"level": "info",
								},
							},
							{
								Label: "AppenderRef",
								Attributes: map[string]string{
									"ref": "Console",
								},
							},
						},
					},
				},
			},
		},
	})
}
