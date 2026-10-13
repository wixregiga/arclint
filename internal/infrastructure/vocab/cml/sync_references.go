package cmlvocab

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/wixregiga/arclint/internal/infrastructure/vocab/cml/syntax"
)

const (
	referenceBoundedContext          = "BoundedContext"
	referenceUserRequirement         = "UserRequirement"
	referenceAggregate               = "Aggregate"
	referenceService                 = "Service"
	referenceServiceOperation        = "ServiceOperation"
	referenceDomainEvent             = "DomainEvent"
	referenceCommandEvent            = "CommandEvent"
	referenceAbstractStakeholder     = "AbstractStakeholder"
	referenceDomainObject            = "DomainObject"
	referenceServiceRepositoryOption = "ServiceRepositoryOption"
	referenceSimpleDomainObject      = "SimpleDomainObject"
	referenceTrait                   = "Trait"
)

type referenceContract struct {
	target   string
	optional bool
}

// These are the cross-reference assignments in the pinned ContextMappingDSL
// and TacticDDDLanguage grammars. Optional means that the enclosing grammar
// clause can disappear while retaining the referencing construct. Required
// operands of authored operations and flows must never silently disappear.
var referenceContracts = map[string]referenceContract{
	"ContextMap.BoundedContexts":                               {referenceBoundedContext, true},
	"BoundedContext.ImplementedDomainParts":                    {"DomainPart", true},
	"BoundedContext.RealizedBoundedContexts":                   {referenceBoundedContext, true},
	"BoundedContext.RefinedBoundedContext":                     {referenceBoundedContext, true},
	"Subdomain.SupportedFeatures":                              {referenceUserRequirement, true},
	"Partnership.Participant1":                                 {referenceBoundedContext, false},
	"Partnership.Participant2":                                 {referenceBoundedContext, false},
	"SharedKernel.Participant1":                                {referenceBoundedContext, false},
	"SharedKernel.Participant2":                                {referenceBoundedContext, false},
	"UpstreamDownstreamRelationship.Upstream":                  {referenceBoundedContext, false},
	"UpstreamDownstreamRelationship.Downstream":                {referenceBoundedContext, false},
	"CustomerSupplierRelationship.Upstream":                    {referenceBoundedContext, false},
	"CustomerSupplierRelationship.Downstream":                  {referenceBoundedContext, false},
	"UpstreamDownstreamRelationship.UpstreamExposedAggregates": {referenceAggregate, true},
	"CustomerSupplierRelationship.UpstreamExposedAggregates":   {referenceAggregate, true},
	"Aggregate.Owner":                                          {referenceBoundedContext, true},
	"Aggregate.UserRequirements":                               {referenceUserRequirement, true},
	"CoordinationStep.BoundedContext":                          {referenceBoundedContext, false},
	"CoordinationStep.Service":                                 {referenceService, false},
	"CoordinationStep.Operation":                               {referenceServiceOperation, false},
	"DomainEventProductionStep.Aggregate":                      {referenceAggregate, true},
	"CommandInvokationStep.Events":                             {referenceDomainEvent, false},
	"EitherCommandOrOperation.Command":                         {referenceCommandEvent, false},
	"EitherCommandOrOperation.Operation":                       {referenceServiceOperation, false},
	"SingleCommandInvokation.Commands":                         {referenceCommandEvent, false},
	"ConcurrentCommandInvokation.Commands":                     {referenceCommandEvent, false},
	"ExclusiveAlternativeCommandInvokation.Commands":           {referenceCommandEvent, false},
	"InclusiveAlternativeCommandInvokation.Commands":           {referenceCommandEvent, false},
	"SingleOperationInvokation.Operations":                     {referenceServiceOperation, false},
	"ConcurrentOperationInvokation.Operations":                 {referenceServiceOperation, false},
	"ExclusiveAlternativeOperationInvokation.Operations":       {referenceServiceOperation, false},
	"InclusiveAlternativeOperationInvokation.Operations":       {referenceServiceOperation, false},
	"SingleEventProduction.Events":                             {referenceDomainEvent, false},
	"MultipleEventProduction.Events":                           {referenceDomainEvent, false},
	"ExclusiveAlternativeEventProduction.Events":               {referenceDomainEvent, false},
	"InclusiveAlternativeEventProduction.Events":               {referenceDomainEvent, false},
	"UserStory.SplittingStory":                                 {"UserStory", true},
	"Stakeholders.Contexts":                                    {referenceBoundedContext, true},
	"ValueRegister.Context":                                    {referenceBoundedContext, true},
	"ValueElicitation.Stakeholder":                             {referenceAbstractStakeholder, false},
	"ValueEpic.Stakeholder":                                    {referenceAbstractStakeholder, false},
	"ValueWeigthing.Stakeholder":                               {referenceAbstractStakeholder, false},
	"Consumer.MessageRoot":                                     {referenceDomainObject, true},
	"Publish.EventType":                                        {"Event", true},
	"StateTransition.From":                                     {"EnumValue", true},
	"TargetState.Value":                                        {"EnumValue", false},
	"ServiceOperationDelegate.Delegate":                        {referenceServiceRepositoryOption, false},
	"ServiceOperationDelegate.DelegateOperation":               {"ServiceRepositoryOperationOption", false},
	"ResourceOperationDelegate.Delegate":                       {referenceService, false},
	"ResourceOperationDelegate.DelegateOperation":              {referenceServiceOperation, false},
	"ComplexType.DomainObjectType":                             {referenceSimpleDomainObject, false},
	"ComplexType.MapKeyDomainObjectType":                       {referenceSimpleDomainObject, false},
	"Entity.Extends":                                           {"Entity", true},
	"ValueObject.Extends":                                      {"ValueObject", true},
	"DomainEvent.Extends":                                      {referenceDomainEvent, true},
	"CommandEvent.Extends":                                     {referenceCommandEvent, true},
	"DataTransferObject.Extends":                               {"DataTransferObject", true},
	"Entity.Traits":                                            {referenceTrait, true},
	"ValueObject.Traits":                                       {referenceTrait, true},
	"DomainEvent.Traits":                                       {referenceTrait, true},
	"CommandEvent.Traits":                                      {referenceTrait, true},
	"BasicType.Traits":                                         {referenceTrait, true},
	"Entity.BelongsTo":                                         {referenceDomainObject, true},
	"ValueObject.BelongsTo":                                    {referenceDomainObject, true},
	"DomainEvent.BelongsTo":                                    {referenceDomainObject, true},
	"CommandEvent.BelongsTo":                                   {referenceDomainObject, true},
	"Reference.DomainObjectType":                               {referenceSimpleDomainObject, false},
	"Association.DomainObjectType":                             {referenceSimpleDomainObject, false},
	"DtoReference.DomainObjectType":                            {referenceSimpleDomainObject, false},
	"OppositeHolder.Opposite":                                  {"Reference", false},
	"ServiceDependency.Dependency":                             {referenceService, false},
	"Dependency.Dependency":                                    {referenceServiceRepositoryOption, false},
}

func referenceTargetMatches(target, kind string) bool {
	switch target {
	case "DomainPart":
		return kind == "Domain" || kind == "Subdomain"
	case referenceDomainObject:
		return kind == "Entity" || kind == "ValueObject" || kind == referenceDomainEvent || kind == referenceCommandEvent
	case referenceSimpleDomainObject:
		return referenceTargetMatches(referenceDomainObject, kind) || kind == "BasicType" || kind == "Enum" || kind == referenceTrait || kind == "DataTransferObject"
	case "Event":
		return kind == referenceDomainEvent || kind == referenceCommandEvent
	case referenceUserRequirement:
		return kind == "UseCase" || kind == "UserStory"
	case referenceAbstractStakeholder:
		return kind == "Stakeholder" || kind == "StakeholderGroup"
	case referenceServiceRepositoryOption:
		return kind == referenceService || kind == "Repository"
	case "ServiceRepositoryOperationOption":
		return kind == referenceServiceOperation || kind == "RepositoryOperation"
	default:
		return target == kind
	}
}

type syntaxDeclaration struct {
	value   reflect.Value
	node    *syntax.Node
	context *syntax.BoundedContext
	parent  *syntax.Node
}

type boundReference struct {
	owner    syntaxDeclaration
	field    string
	contract referenceContract
	targets  map[string]*syntax.Node
}

func syntaxDeclarations(file *syntax.File) []syntaxDeclaration {
	var out []syntaxDeclaration
	var walk func(reflect.Value, *syntax.BoundedContext, *syntax.Node)
	walk = func(v reflect.Value, context *syntax.BoundedContext, parent *syntax.Node) {
		if !v.IsValid() {
			return
		}
		if v.Kind() == reflect.Interface {
			if !v.IsNil() {
				walk(v.Elem(), context, parent)
			}
			return
		}
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return
			}
			if bc, ok := v.Interface().(*syntax.BoundedContext); ok {
				context = bc
			}
			if node := syntaxNode(v.Interface()); node != nil {
				out = append(out, syntaxDeclaration{v.Elem(), node, context, parent})
				parent = node
			}
			walk(v.Elem(), context, parent)
			return
		}
		switch v.Kind() {
		default:
			return
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() && v.Type().Field(i).Name != nodeField {
					walk(v.Field(i), context, parent)
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i), context, parent)
			}
		}
	}
	walk(reflect.ValueOf(file), nil, nil)
	return out
}

func bindReferences(file *syntax.File) []boundReference {
	declarations := syntaxDeclarations(file)
	var out []boundReference
	for _, owner := range declarations {
		for i := 0; i < owner.value.NumField(); i++ {
			field := owner.value.Type().Field(i).Name
			contract, ok := referenceContracts[owner.value.Type().Name()+"."+field]
			if !ok {
				continue
			}
			refs := owner.value.Field(i)
			var names []string
			if refs.Type() == reflect.TypeOf(syntax.Ref{}) {
				names = []string{refs.Interface().(syntax.Ref).Name}
			} else {
				for j := 0; j < refs.Len(); j++ {
					names = append(names, refs.Index(j).Interface().(syntax.Ref).Name)
				}
			}
			bound := boundReference{owner, field, contract, map[string]*syntax.Node{}}
			for _, name := range names {
				if name == "" {
					continue
				}
				if target := resolveReference(owner, field, contract, name, declarations); target != nil {
					bound.targets[name] = target.node
				}
			}
			out = append(out, bound)
		}
	}
	return out
}

func repairReferences(file *syntax.File, bound []boundReference, path string) error {
	live := liveDeclarations(file)
	for _, reference := range bound {
		if _, ok := live[reference.owner.node]; !ok {
			continue
		}
		field := reference.owner.value.FieldByName(reference.field)
		repair := func(ref syntax.Ref) (syntax.Ref, error) {
			target := reference.targets[ref.Name]
			if target == nil {
				return ref, nil
			}
			declaration, exists := live[target]
			if exists {
				ref.Name = declaration.value.FieldByName("Name").String()
				return ref, nil
			}
			if !reference.contract.optional {
				line := ref.Pos.Line
				if line == 0 {
					line = reference.owner.node.Pos.Line
				}
				return ref, fmt.Errorf("%s:%d: %s.%s requires %s %q; its declaration was removed and this reference cannot be dropped", path, line, reference.owner.value.Type().Name(), reference.field, reference.contract.target, ref.Name)
			}
			return syntax.Ref{}, nil
		}
		if field.Type() == reflect.TypeOf(syntax.Ref{}) {
			ref, err := repair(field.Interface().(syntax.Ref))
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(ref))
			if clearRemovedDelegation(reference, ref) {
				live = liveDeclarations(file)
			}
		} else {
			if err := repairReferenceList(field, repair); err != nil {
				return err
			}
		}
		// Empty optional assignments must not retain an explicit-presence marker.
		if field.IsZero() || field.Kind() == reflect.Slice && field.Len() == 0 {
			delete(reference.owner.node.Present, strings.ToLower(reference.field[:1])+reference.field[1:])
		}
	}
	return nil
}

func clearRemovedDelegation(reference boundReference, ref syntax.Ref) bool {
	if ref.Name == "" && reference.owner.value.Type().Name() == "DomainEventProductionStep" && reference.field == "Aggregate" {
		reference.owner.value.FieldByName("StateTransition").SetZero()
		return true
	}
	return false
}

func liveDeclarations(file *syntax.File) map[*syntax.Node]syntaxDeclaration {
	live := map[*syntax.Node]syntaxDeclaration{}
	for _, declaration := range syntaxDeclarations(file) {
		live[declaration.node] = declaration
	}
	return live
}

func resolveReference(owner syntaxDeclaration, field string, contract referenceContract, name string, declarations []syntaxDeclaration) *syntaxDeclaration {
	context := owner.context
	var parentName string
	qualified := false
	if owner.value.Type().Name() == "CoordinationStep" && field != referenceBoundedContext {
		qualified = true
		contextName := owner.value.FieldByName(referenceBoundedContext).Interface().(syntax.Ref).Name
		context = nil
		for _, declaration := range declarations {
			if declaration.context != nil && declaration.context.Name == contextName {
				context = declaration.context
				break
			}
		}
		if field == "Operation" {
			parentName = owner.value.FieldByName(referenceService).Interface().(syntax.Ref).Name
		}
	}
	if field == "DelegateOperation" {
		parentName = owner.value.FieldByName("Delegate").Interface().(syntax.Ref).Name
		qualified = true
	}
	var best *syntaxDeclaration
	for i := range declarations {
		candidate := &declarations[i]
		candidateName := candidate.value.FieldByName("Name")
		if !candidateName.IsValid() || candidateName.String() != name || !referenceTargetMatches(contract.target, candidate.value.Type().Name()) {
			continue
		}
		if qualified && candidate.context != context {
			continue
		}
		if parentName != "" {
			matchesParent := false
			for _, parent := range declarations {
				if parent.node == candidate.parent {
					n := parent.value.FieldByName("Name")
					matchesParent = n.IsValid() && n.String() == parentName
					break
				}
			}
			if !matchesParent {
				continue
			}
		}
		if best == nil || candidate.context == context && best.context != context {
			best = candidate
		}
	}
	return best
}

func repairReferenceList(field reflect.Value, repair func(syntax.Ref) (syntax.Ref, error)) error {
	refs := make([]syntax.Ref, 0, field.Len())
	for i := 0; i < field.Len(); i++ {
		ref, err := repair(field.Index(i).Interface().(syntax.Ref))
		if err != nil {
			return err
		}
		if ref.Name != "" {
			refs = append(refs, ref)
		}
	}
	field.Set(reflect.ValueOf(refs))
	return nil
}
