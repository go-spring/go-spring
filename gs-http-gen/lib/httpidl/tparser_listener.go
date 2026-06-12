// Code generated from TParser.g4 by ANTLR 4.13.2. DO NOT EDIT.

package httpidl // TParser
import "github.com/antlr4-go/antlr/v4"

// TParserListener is a complete listener for a parse tree produced by TParser.
type TParserListener interface {
	antlr.ParseTreeListener

	// EnterDocument is called when entering the document production.
	EnterDocument(c *DocumentContext)

	// EnterDefinition is called when entering the definition production.
	EnterDefinition(c *DefinitionContext)

	// EnterConst_def is called when entering the const_def production.
	EnterConst_def(c *Const_defContext)

	// EnterEnum_def is called when entering the enum_def production.
	EnterEnum_def(c *Enum_defContext)

	// EnterEnum_field is called when entering the enum_field production.
	EnterEnum_field(c *Enum_fieldContext)

	// EnterType_def is called when entering the type_def production.
	EnterType_def(c *Type_defContext)

	// EnterType_field is called when entering the type_field production.
	EnterType_field(c *Type_fieldContext)

	// EnterEmbed_type_field is called when entering the embed_type_field production.
	EnterEmbed_type_field(c *Embed_type_fieldContext)

	// EnterCommon_type_field is called when entering the common_type_field production.
	EnterCommon_type_field(c *Common_type_fieldContext)

	// EnterField_annotations is called when entering the field_annotations production.
	EnterField_annotations(c *Field_annotationsContext)

	// EnterOneof_def is called when entering the oneof_def production.
	EnterOneof_def(c *Oneof_defContext)

	// EnterRpc_def is called when entering the rpc_def production.
	EnterRpc_def(c *Rpc_defContext)

	// EnterRpc_req is called when entering the rpc_req production.
	EnterRpc_req(c *Rpc_reqContext)

	// EnterRpc_resp is called when entering the rpc_resp production.
	EnterRpc_resp(c *Rpc_respContext)

	// EnterRpc_annotations is called when entering the rpc_annotations production.
	EnterRpc_annotations(c *Rpc_annotationsContext)

	// EnterAnnotation is called when entering the annotation production.
	EnterAnnotation(c *AnnotationContext)

	// EnterBase_type is called when entering the base_type production.
	EnterBase_type(c *Base_typeContext)

	// EnterUser_type is called when entering the user_type production.
	EnterUser_type(c *User_typeContext)

	// EnterContainer_type is called when entering the container_type production.
	EnterContainer_type(c *Container_typeContext)

	// EnterMap_type is called when entering the map_type production.
	EnterMap_type(c *Map_typeContext)

	// EnterKey_type is called when entering the key_type production.
	EnterKey_type(c *Key_typeContext)

	// EnterList_type is called when entering the list_type production.
	EnterList_type(c *List_typeContext)

	// EnterValue_type is called when entering the value_type production.
	EnterValue_type(c *Value_typeContext)

	// EnterConst_value is called when entering the const_value production.
	EnterConst_value(c *Const_valueContext)

	// EnterTerminator is called when entering the terminator production.
	EnterTerminator(c *TerminatorContext)

	// ExitDocument is called when exiting the document production.
	ExitDocument(c *DocumentContext)

	// ExitDefinition is called when exiting the definition production.
	ExitDefinition(c *DefinitionContext)

	// ExitConst_def is called when exiting the const_def production.
	ExitConst_def(c *Const_defContext)

	// ExitEnum_def is called when exiting the enum_def production.
	ExitEnum_def(c *Enum_defContext)

	// ExitEnum_field is called when exiting the enum_field production.
	ExitEnum_field(c *Enum_fieldContext)

	// ExitType_def is called when exiting the type_def production.
	ExitType_def(c *Type_defContext)

	// ExitType_field is called when exiting the type_field production.
	ExitType_field(c *Type_fieldContext)

	// ExitEmbed_type_field is called when exiting the embed_type_field production.
	ExitEmbed_type_field(c *Embed_type_fieldContext)

	// ExitCommon_type_field is called when exiting the common_type_field production.
	ExitCommon_type_field(c *Common_type_fieldContext)

	// ExitField_annotations is called when exiting the field_annotations production.
	ExitField_annotations(c *Field_annotationsContext)

	// ExitOneof_def is called when exiting the oneof_def production.
	ExitOneof_def(c *Oneof_defContext)

	// ExitRpc_def is called when exiting the rpc_def production.
	ExitRpc_def(c *Rpc_defContext)

	// ExitRpc_req is called when exiting the rpc_req production.
	ExitRpc_req(c *Rpc_reqContext)

	// ExitRpc_resp is called when exiting the rpc_resp production.
	ExitRpc_resp(c *Rpc_respContext)

	// ExitRpc_annotations is called when exiting the rpc_annotations production.
	ExitRpc_annotations(c *Rpc_annotationsContext)

	// ExitAnnotation is called when exiting the annotation production.
	ExitAnnotation(c *AnnotationContext)

	// ExitBase_type is called when exiting the base_type production.
	ExitBase_type(c *Base_typeContext)

	// ExitUser_type is called when exiting the user_type production.
	ExitUser_type(c *User_typeContext)

	// ExitContainer_type is called when exiting the container_type production.
	ExitContainer_type(c *Container_typeContext)

	// ExitMap_type is called when exiting the map_type production.
	ExitMap_type(c *Map_typeContext)

	// ExitKey_type is called when exiting the key_type production.
	ExitKey_type(c *Key_typeContext)

	// ExitList_type is called when exiting the list_type production.
	ExitList_type(c *List_typeContext)

	// ExitValue_type is called when exiting the value_type production.
	ExitValue_type(c *Value_typeContext)

	// ExitConst_value is called when exiting the const_value production.
	ExitConst_value(c *Const_valueContext)

	// ExitTerminator is called when exiting the terminator production.
	ExitTerminator(c *TerminatorContext)
}
