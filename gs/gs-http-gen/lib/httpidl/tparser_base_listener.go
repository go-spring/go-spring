// Code generated from TParser.g4 by ANTLR 4.13.2. DO NOT EDIT.

package httpidl // TParser
import "github.com/antlr4-go/antlr/v4"

// BaseTParserListener is a complete listener for a parse tree produced by TParser.
type BaseTParserListener struct{}

var _ TParserListener = &BaseTParserListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseTParserListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseTParserListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseTParserListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseTParserListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterDocument is called when production document is entered.
func (s *BaseTParserListener) EnterDocument(ctx *DocumentContext) {}

// ExitDocument is called when production document is exited.
func (s *BaseTParserListener) ExitDocument(ctx *DocumentContext) {}

// EnterDefinition is called when production definition is entered.
func (s *BaseTParserListener) EnterDefinition(ctx *DefinitionContext) {}

// ExitDefinition is called when production definition is exited.
func (s *BaseTParserListener) ExitDefinition(ctx *DefinitionContext) {}

// EnterConst_def is called when production const_def is entered.
func (s *BaseTParserListener) EnterConst_def(ctx *Const_defContext) {}

// ExitConst_def is called when production const_def is exited.
func (s *BaseTParserListener) ExitConst_def(ctx *Const_defContext) {}

// EnterEnum_def is called when production enum_def is entered.
func (s *BaseTParserListener) EnterEnum_def(ctx *Enum_defContext) {}

// ExitEnum_def is called when production enum_def is exited.
func (s *BaseTParserListener) ExitEnum_def(ctx *Enum_defContext) {}

// EnterEnum_field is called when production enum_field is entered.
func (s *BaseTParserListener) EnterEnum_field(ctx *Enum_fieldContext) {}

// ExitEnum_field is called when production enum_field is exited.
func (s *BaseTParserListener) ExitEnum_field(ctx *Enum_fieldContext) {}

// EnterType_def is called when production type_def is entered.
func (s *BaseTParserListener) EnterType_def(ctx *Type_defContext) {}

// ExitType_def is called when production type_def is exited.
func (s *BaseTParserListener) ExitType_def(ctx *Type_defContext) {}

// EnterType_field is called when production type_field is entered.
func (s *BaseTParserListener) EnterType_field(ctx *Type_fieldContext) {}

// ExitType_field is called when production type_field is exited.
func (s *BaseTParserListener) ExitType_field(ctx *Type_fieldContext) {}

// EnterEmbed_type_field is called when production embed_type_field is entered.
func (s *BaseTParserListener) EnterEmbed_type_field(ctx *Embed_type_fieldContext) {}

// ExitEmbed_type_field is called when production embed_type_field is exited.
func (s *BaseTParserListener) ExitEmbed_type_field(ctx *Embed_type_fieldContext) {}

// EnterCommon_type_field is called when production common_type_field is entered.
func (s *BaseTParserListener) EnterCommon_type_field(ctx *Common_type_fieldContext) {}

// ExitCommon_type_field is called when production common_type_field is exited.
func (s *BaseTParserListener) ExitCommon_type_field(ctx *Common_type_fieldContext) {}

// EnterField_annotations is called when production field_annotations is entered.
func (s *BaseTParserListener) EnterField_annotations(ctx *Field_annotationsContext) {}

// ExitField_annotations is called when production field_annotations is exited.
func (s *BaseTParserListener) ExitField_annotations(ctx *Field_annotationsContext) {}

// EnterOneof_def is called when production oneof_def is entered.
func (s *BaseTParserListener) EnterOneof_def(ctx *Oneof_defContext) {}

// ExitOneof_def is called when production oneof_def is exited.
func (s *BaseTParserListener) ExitOneof_def(ctx *Oneof_defContext) {}

// EnterRpc_def is called when production rpc_def is entered.
func (s *BaseTParserListener) EnterRpc_def(ctx *Rpc_defContext) {}

// ExitRpc_def is called when production rpc_def is exited.
func (s *BaseTParserListener) ExitRpc_def(ctx *Rpc_defContext) {}

// EnterRpc_req is called when production rpc_req is entered.
func (s *BaseTParserListener) EnterRpc_req(ctx *Rpc_reqContext) {}

// ExitRpc_req is called when production rpc_req is exited.
func (s *BaseTParserListener) ExitRpc_req(ctx *Rpc_reqContext) {}

// EnterRpc_resp is called when production rpc_resp is entered.
func (s *BaseTParserListener) EnterRpc_resp(ctx *Rpc_respContext) {}

// ExitRpc_resp is called when production rpc_resp is exited.
func (s *BaseTParserListener) ExitRpc_resp(ctx *Rpc_respContext) {}

// EnterRpc_annotations is called when production rpc_annotations is entered.
func (s *BaseTParserListener) EnterRpc_annotations(ctx *Rpc_annotationsContext) {}

// ExitRpc_annotations is called when production rpc_annotations is exited.
func (s *BaseTParserListener) ExitRpc_annotations(ctx *Rpc_annotationsContext) {}

// EnterAnnotation is called when production annotation is entered.
func (s *BaseTParserListener) EnterAnnotation(ctx *AnnotationContext) {}

// ExitAnnotation is called when production annotation is exited.
func (s *BaseTParserListener) ExitAnnotation(ctx *AnnotationContext) {}

// EnterBase_type is called when production base_type is entered.
func (s *BaseTParserListener) EnterBase_type(ctx *Base_typeContext) {}

// ExitBase_type is called when production base_type is exited.
func (s *BaseTParserListener) ExitBase_type(ctx *Base_typeContext) {}

// EnterUser_type is called when production user_type is entered.
func (s *BaseTParserListener) EnterUser_type(ctx *User_typeContext) {}

// ExitUser_type is called when production user_type is exited.
func (s *BaseTParserListener) ExitUser_type(ctx *User_typeContext) {}

// EnterContainer_type is called when production container_type is entered.
func (s *BaseTParserListener) EnterContainer_type(ctx *Container_typeContext) {}

// ExitContainer_type is called when production container_type is exited.
func (s *BaseTParserListener) ExitContainer_type(ctx *Container_typeContext) {}

// EnterMap_type is called when production map_type is entered.
func (s *BaseTParserListener) EnterMap_type(ctx *Map_typeContext) {}

// ExitMap_type is called when production map_type is exited.
func (s *BaseTParserListener) ExitMap_type(ctx *Map_typeContext) {}

// EnterKey_type is called when production key_type is entered.
func (s *BaseTParserListener) EnterKey_type(ctx *Key_typeContext) {}

// ExitKey_type is called when production key_type is exited.
func (s *BaseTParserListener) ExitKey_type(ctx *Key_typeContext) {}

// EnterList_type is called when production list_type is entered.
func (s *BaseTParserListener) EnterList_type(ctx *List_typeContext) {}

// ExitList_type is called when production list_type is exited.
func (s *BaseTParserListener) ExitList_type(ctx *List_typeContext) {}

// EnterValue_type is called when production value_type is entered.
func (s *BaseTParserListener) EnterValue_type(ctx *Value_typeContext) {}

// ExitValue_type is called when production value_type is exited.
func (s *BaseTParserListener) ExitValue_type(ctx *Value_typeContext) {}

// EnterConst_value is called when production const_value is entered.
func (s *BaseTParserListener) EnterConst_value(ctx *Const_valueContext) {}

// ExitConst_value is called when production const_value is exited.
func (s *BaseTParserListener) ExitConst_value(ctx *Const_valueContext) {}

// EnterTerminator is called when production terminator is entered.
func (s *BaseTParserListener) EnterTerminator(ctx *TerminatorContext) {}

// ExitTerminator is called when production terminator is exited.
func (s *BaseTParserListener) ExitTerminator(ctx *TerminatorContext) {}
