package syntax

import (
	"fmt"
	"reflect"
)

var nodeTypes = func() map[string]reflect.Type {
	result := map[string]reflect.Type{}
	for _, node := range []any{
		ContextMappingModel{}, Import{}, ContextMap{}, BoundedContext{}, Domain{}, Subdomain{}, Partnership{}, SharedKernel{}, UpstreamDownstreamRelationship{}, CustomerSupplierRelationship{}, Aggregate{}, Application{}, Coordination{}, CoordinationStep{}, Flow{}, DomainEventProductionStep{}, CommandInvokationStep{}, EitherCommandOrOperation{}, SingleCommandInvokation{}, ConcurrentCommandInvokation{}, ExclusiveAlternativeCommandInvokation{}, InclusiveAlternativeCommandInvokation{}, SingleOperationInvokation{}, ConcurrentOperationInvokation{}, ExclusiveAlternativeOperationInvokation{}, InclusiveAlternativeOperationInvokation{}, SingleEventProduction{}, MultipleEventProduction{}, ExclusiveAlternativeEventProduction{}, InclusiveAlternativeEventProduction{}, UseCase{}, UserStory{}, NormalFeature{}, StoryFeature{}, StoryValuation{}, SculptorModule{}, Stakeholders{}, StakeholderGroup{}, Stakeholder{}, ValueRegister{}, ValueCluster{}, Value{}, ValueElicitation{}, ValueEpic{}, ValueNarrative{}, ValueWeigthing{}, Consequence{}, Action{}, TacticDDDModel{}, TacticDDDApplication{}, Service{}, Resource{}, Consumer{}, Subscribe{}, Publish{}, ServiceOperation{}, StateTransition{}, SingleStateTransitionTarget{}, ExclusiveAlternativeStateTransitionTarget{}, TargetState{}, ServiceOperationDelegate{}, ResourceOperation{}, ResourceOperationDelegate{}, RepositoryOperation{}, Parameter{}, ComplexType{}, Entity{}, ValueObject{}, DomainEvent{}, CommandEvent{}, Trait{}, DomainObjectOperation{}, DataTransferObject{}, BasicType{}, Attribute{}, Reference{}, Association{}, DtoAttribute{}, DtoReference{}, OppositeHolder{}, Repository{}, ServiceDependency{}, Dependency{}, Enum{}, EnumAttribute{}, EnumValue{}, EnumParameter{},
	} {
		t := reflect.TypeOf(node)
		result[t.Name()] = t
	}
	return result
}()

func nodeOf(value any) *Node {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return nil
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return nil
	}
	field := v.FieldByName(nodeField)
	if !field.IsValid() {
		return nil
	}
	return field.Addr().Interface().(*Node)
}

func assignFeature(v reflect.Value, name string, value any, appendValue bool) error {
	field := v.FieldByName(goFeature(name))
	if !field.IsValid() {
		return fmt.Errorf("%s has no field for grammar feature %s", v.Type(), name)
	}
	x := reflect.ValueOf(value)
	target := field.Type()
	if appendValue {
		target = target.Elem()
	}
	if target == reflect.TypeOf((*ComplexType)(nil)) && x.Kind() == reflect.String {
		x = reflect.ValueOf(&ComplexType{Type: x.String()})
	}
	if !x.Type().AssignableTo(target) {
		if !x.Type().ConvertibleTo(target) {
			return fmt.Errorf("cannot assign %T to %s.%s", value, v.Type(), name)
		}
		x = x.Convert(target)
	}

	if appendValue {
		field.Set(reflect.Append(field, x))
	} else {
		field.Set(x)
	}
	return nil
}

// Grammar unions are checked through their sealed Go interfaces before a value
// is assigned to its caller. This also catches registry mistakes at their rule.
func unionValue(rule string, value any) (any, error) {
	switch rule {
	case "DomainPart":
		return requireMember[DomainPart](rule, value, func(member DomainPart) *Node { return member.domainPart() })
	case "Relationship":
		return requireMember[Relationship](rule, value, func(member Relationship) *Node { return member.relationship() })
	case "SymmetricRelationship":
		return requireMember[SymmetricRelationship](rule, value, func(member SymmetricRelationship) *Node { return member.symmetricRelationship() })
	case "FlowStep":
		return requireMember[FlowStep](rule, value, func(member FlowStep) *Node { return member.flowStep() })
	case "EitherCommandOrOperationInvokation":
		return requireMember[EitherCommandOrOperationInvokation](rule, value, func(member EitherCommandOrOperationInvokation) *Node {
			return member.eitherCommandOrOperationInvokation()
		})
	case "CommandInvokation":
		return requireMember[CommandInvokation](rule, value, func(member CommandInvokation) *Node { return member.commandInvokation() })
	case "OperationInvokation":
		return requireMember[OperationInvokation](rule, value, func(member OperationInvokation) *Node { return member.operationInvokation() })
	case "EventProduction":
		return requireMember[EventProduction](rule, value, func(member EventProduction) *Node { return member.eventProduction() })
	case "UserRequirement":
		return requireMember[UserRequirement](rule, value, func(member UserRequirement) *Node { return member.userRequirement() })
	case "Feature":
		return requireMember[Feature](rule, value, func(member Feature) *Node { return member.feature() })
	case "AbstractStakeholder":
		return requireMember[AbstractStakeholder](rule, value, func(member AbstractStakeholder) *Node { return member.abstractStakeholder() })
	case "Event":
		return requireMember[Event](rule, value, func(member Event) *Node { return member.event() })
	case "DomainObjectTypedElement":
		return requireMember[DomainObjectTypedElement](rule, value, func(member DomainObjectTypedElement) *Node { return member.domainObjectTypedElement() })
	case "StateTransitionTarget":
		return requireMember[StateTransitionTarget](rule, value, func(member StateTransitionTarget) *Node { return member.stateTransitionTarget() })
	case "ServiceRepositoryOption":
		return requireMember[ServiceRepositoryOption](rule, value, func(member ServiceRepositoryOption) *Node { return member.serviceRepositoryOption() })
	case "ServiceRepositoryOperationOption":
		return requireMember[ServiceRepositoryOperationOption](rule, value, func(member ServiceRepositoryOperationOption) *Node { return member.serviceRepositoryOperationOption() })
	case "SimpleDomainObject":
		return requireMember[SimpleDomainObject](rule, value, func(member SimpleDomainObject) *Node { return member.simpleDomainObject() })
	case "DomainObject":
		return requireMember[DomainObject](rule, value, func(member DomainObject) *Node { return member.domainObject() })
	case "AnyProperty":
		return requireMember[AnyProperty](rule, value, func(member AnyProperty) *Node { return member.anyProperty() })
	case "Property":
		return requireMember[Property](rule, value, func(member Property) *Node { return member.property() })
	case "DtoProperty":
		return requireMember[DtoProperty](rule, value, func(member DtoProperty) *Node { return member.dtoProperty() })
	}
	return nil, fmt.Errorf("unknown grammar union %s", rule)
}

func requireMember[T any](rule string, value any, origin func(T) *Node) (any, error) {
	member, ok := value.(T)
	if !ok {
		return nil, fmt.Errorf("%T is not a member of %s", value, rule)
	}
	if origin(member) == nil {
		return nil, fmt.Errorf("nil member of %s", rule)
	}
	return member, nil
}
