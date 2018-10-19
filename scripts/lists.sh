#!/usr/bin/env bash

typeNames=(
  Argument
  Definition
  Directive
  DirectiveLocation
  EnumValueDefinition
  Error
  FieldDefinition
  InputValueDefinition
  OperationTypeDefinition
  RootOperationTypeDefinition
  Selection
  Type
  VariableDefinition
)

types=
for i in ${!typeNames[@]}; do
  types+=${typeNames[$i]}

  if [ $(expr ${i} + 1) -lt ${#typeNames[@]} ]; then
    types+=,
  fi
done

go run lab/generics/main.go -package ast -types ${types} > ast/lists.go
go fmt ast/lists.go
