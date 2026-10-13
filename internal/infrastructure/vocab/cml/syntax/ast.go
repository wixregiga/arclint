package syntax

// Pos identifies a token using a zero-based byte offset and one-based line and column.
type Pos struct{ Offset, Line, Column int }

// Comment retains a complete line or block comment, including its delimiters.
type Comment struct {
	Text string
	Pos  Pos
}

// Node carries source provenance and comments. NamePos is zero for unnamed nodes.
type Node struct {
	// ExplicitBody retains the braces of an optional body that the author wrote
	// around nothing, or around optional syntax no assigned field represents (a
	// keyword kept at its default). Braces around assigned fields alone print
	// while one of them has a value and go with the last. Clear it to omit any
	// other emptied body; a body with content always prints its braces.
	ExplicitBody     bool
	Pos              Pos
	NamePos          Pos
	LeadingComments  []Comment
	TrailingComments []Comment
	// Present distinguishes explicitly assigned zero values from absent features.
	// Names use the grammar's lower camel case, for example "doc" or "integerValue".
	Present map[string]bool
	// Spellings retains escaped datatype identifiers separately from their value.
	Spellings map[string]string
	form      *derivation
	choices   map[choiceKey][]syntaxChoice
}

// Ref is an unresolved cross-reference. Parsing never performs linking.
type Ref struct {
	Name string
	Pos  Pos
}

// File is the complete context mapping model.
type File = ContextMappingModel

// ContextMappingModel represents the ContextMappingModel grammar rule.
type ContextMappingModel struct {
	Node
	TopComment       string
	Imports          []*Import
	Map              *ContextMap
	BoundedContexts  []*BoundedContext
	Domains          []*Domain
	UserRequirements []UserRequirement
	Stakeholders     []*Stakeholders
	ValueRegisters   []*ValueRegister
}

// Import represents the Import grammar rule.
type Import struct {
	Node
	ImportURI string
}

// ContextMap represents the ContextMap grammar rule.
type ContextMap struct {
	Node
	Name            string
	Type            ContextMapType
	State           ContextMapState
	BoundedContexts []Ref
	Relationships   []Relationship
}

// BoundedContext represents the BoundedContext grammar rule.
type BoundedContext struct {
	Node
	Comment                  string
	Name                     string
	ImplementedDomainParts   []Ref
	RealizedBoundedContexts  []Ref
	RefinedBoundedContext    Ref
	DomainVisionStatement    string
	Type                     BoundedContextType
	Responsibilities         []string
	ImplementationTechnology string
	KnowledgeLevel           KnowledgeLevel
	BusinessModel            string
	Evolution                Evolution
	Application              *Application
	Modules                  []*SculptorModule
	Aggregates               []*Aggregate
	DomainServices           []*Service
}

// DomainPart represents the DomainPart grammar rule.
type DomainPart interface{ domainPart() *Node }

// Domain represents the Domain grammar rule.
type Domain struct {
	Node
	Name                  string
	DomainVisionStatement string
	Subdomains            []*Subdomain
}

// Subdomain represents the Subdomain grammar rule.
type Subdomain struct {
	Node
	Name                  string
	SupportedFeatures     []Ref
	Type                  SubDomainType
	DomainVisionStatement string
	Entities              []*Entity
	Services              []*Service
}

// Relationship represents the Relationship grammar rule.
type Relationship interface{ relationship() *Node }

// SymmetricRelationship represents the SymmetricRelationship grammar rule.
type SymmetricRelationship interface{ symmetricRelationship() *Node }

// Notation identifies the grammar alternative, numbered in grammar order from zero.
// It preserves participant order and bracket placement. For new relationships,
// use the keyword alternatives: Partnership 4, SharedKernel 4, directional 2 or 3.
type Notation int

// Partnership represents the Partnership grammar rule.
type Partnership struct {
	Node
	Participant1             Ref
	Participant2             Ref
	Name                     string
	ImplementationTechnology string
	Notation                 Notation
}

// SharedKernel represents the SharedKernel grammar rule.
type SharedKernel struct {
	Node
	Participant1             Ref
	Participant2             Ref
	Name                     string
	ImplementationTechnology string
	Notation                 Notation
}

// UpstreamDownstreamRelationship represents the UpstreamDownstreamRelationship grammar rule.
type UpstreamDownstreamRelationship struct {
	Node
	Upstream                   Ref
	Downstream                 Ref
	UpstreamRoles              []UpstreamRole
	DownstreamRoles            []DownstreamRole
	Name                       string
	ImplementationTechnology   string
	UpstreamExposedAggregates  []Ref
	ExposedAggregatesComment   string
	DownstreamGovernanceRights DownstreamGovernanceRights
	Notation                   Notation
	// Bracket and marker flags retain optional lexical choices in the header.
	UpstreamBracket      bool
	DownstreamBracket    bool
	UpstreamRoleMarker   bool
	DownstreamRoleMarker bool
}

// CustomerSupplierRelationship represents the CustomerSupplierRelationship grammar rule.
type CustomerSupplierRelationship struct {
	Node
	Upstream                   Ref
	Downstream                 Ref
	UpstreamRoles              []UpstreamRole
	DownstreamRoles            []DownstreamRole
	Name                       string
	ImplementationTechnology   string
	UpstreamExposedAggregates  []Ref
	ExposedAggregatesComment   string
	DownstreamGovernanceRights DownstreamGovernanceRights
	Notation                   Notation
	// Bracket and marker flags retain optional lexical choices in the header.
	UpstreamBracket      bool
	DownstreamBracket    bool
	UpstreamRoleMarker   bool
	DownstreamRoleMarker bool
}

// Aggregate represents the Aggregate grammar rule.
type Aggregate struct {
	Node
	Comment                 string
	Doc                     string
	Name                    string
	Responsibilities        []string
	UserRequirements        []Ref
	Owner                   Ref
	KnowledgeLevel          KnowledgeLevel
	LikelihoodForChange     Volatility
	ContentVolatility       Volatility
	AvailabilityCriticality Criticality
	ConsistencyCriticality  Criticality
	StorageSimilarity       Similarity
	SecurityCriticality     Criticality
	SecurityZone            string
	SecurityAccessGroup     string
	Services                []*Service
	Resources               []*Resource
	Consumers               []*Consumer
	DomainObjects           []SimpleDomainObject
}

// Application represents the Application grammar rule.
type Application struct {
	Node
	Name          string
	Commands      []*CommandEvent
	Events        []*DomainEvent
	Services      []*Service
	Flows         []*Flow
	Coordinations []*Coordination
}

// Coordination represents the Coordination grammar rule.
type Coordination struct {
	Node
	Name              string
	CoordinationSteps []*CoordinationStep
}

// CoordinationStep represents the CoordinationStep grammar rule.
type CoordinationStep struct {
	Node
	BoundedContext Ref
	Service        Ref
	Operation      Ref
}

// Flow represents the Flow grammar rule.
type Flow struct {
	Node
	Name  string
	Steps []FlowStep
}

// FlowStep represents the FlowStep grammar rule.
type FlowStep interface{ flowStep() *Node }

// DomainEventProductionStep represents the DomainEventProductionStep grammar rule.
type DomainEventProductionStep struct {
	Node
	Action          *EitherCommandOrOperation
	Aggregate       Ref
	StateTransition *StateTransition
	EventProduction EventProduction
}

// CommandInvokationStep represents the CommandInvokationStep grammar rule.
type CommandInvokationStep struct {
	Node
	Events []Ref
	Action EitherCommandOrOperationInvokation
}

// EitherCommandOrOperation represents the EitherCommandOrOperation grammar rule.
type EitherCommandOrOperation struct {
	Node
	Command   Ref
	Operation Ref
	Actor     string
}

// EitherCommandOrOperationInvokation represents the EitherCommandOrOperationInvokation grammar rule.
type EitherCommandOrOperationInvokation interface{ eitherCommandOrOperationInvokation() *Node }

// CommandInvokation represents the CommandInvokation grammar rule.
type CommandInvokation interface{ commandInvokation() *Node }

// OperationInvokation represents the OperationInvokation grammar rule.
type OperationInvokation interface{ operationInvokation() *Node }

// SingleCommandInvokation represents the SingleCommandInvokation grammar rule.
type SingleCommandInvokation struct {
	Node
	Commands []Ref
}

// ConcurrentCommandInvokation represents the ConcurrentCommandInvokation grammar rule.
type ConcurrentCommandInvokation struct {
	Node
	Commands []Ref
}

// ExclusiveAlternativeCommandInvokation represents the ExclusiveAlternativeCommandInvokation grammar rule.
type ExclusiveAlternativeCommandInvokation struct {
	Node
	Commands []Ref
}

// InclusiveAlternativeCommandInvokation represents the InclusiveAlternativeCommandInvokation grammar rule.
type InclusiveAlternativeCommandInvokation struct {
	Node
	Commands []Ref
}

// SingleOperationInvokation represents the SingleOperationInvokation grammar rule.
type SingleOperationInvokation struct {
	Node
	Operations []Ref
}

// ConcurrentOperationInvokation represents the ConcurrentOperationInvokation grammar rule.
type ConcurrentOperationInvokation struct {
	Node
	Operations []Ref
}

// ExclusiveAlternativeOperationInvokation represents the ExclusiveAlternativeOperationInvokation grammar rule.
type ExclusiveAlternativeOperationInvokation struct {
	Node
	Operations []Ref
}

// InclusiveAlternativeOperationInvokation represents the InclusiveAlternativeOperationInvokation grammar rule.
type InclusiveAlternativeOperationInvokation struct {
	Node
	Operations []Ref
}

// EventProduction represents the EventProduction grammar rule.
type EventProduction interface{ eventProduction() *Node }

// SingleEventProduction represents the SingleEventProduction grammar rule.
type SingleEventProduction struct {
	Node
	Events []Ref
}

// MultipleEventProduction represents the MultipleEventProduction grammar rule.
type MultipleEventProduction struct {
	Node
	Events []Ref
}

// ExclusiveAlternativeEventProduction represents the ExclusiveAlternativeEventProduction grammar rule.
type ExclusiveAlternativeEventProduction struct {
	Node
	Events []Ref
}

// InclusiveAlternativeEventProduction represents the InclusiveAlternativeEventProduction grammar rule.
type InclusiveAlternativeEventProduction struct {
	Node
	Events []Ref
}

// UserRequirement represents the UserRequirement grammar rule.
type UserRequirement interface{ userRequirement() *Node }

// UseCase represents the UseCase grammar rule.
type UseCase struct {
	Node
	Name            string
	Role            string
	SecondaryActors []string
	Features        []Feature
	Benefit         string
	Scope           string
	Level           string
}

// UserStory represents the UserStory grammar rule.
type UserStory struct {
	Node
	Name           string
	SplittingStory Ref
	Role           string
	Features       []Feature
	Benefit        string
	Valuation      *StoryValuation
}

// Feature represents the Feature grammar rule.
type Feature interface{ feature() *Node }

// NormalFeature represents the NormalFeature grammar rule.
type NormalFeature struct {
	Node
	Verb                        string
	EntityArticle               string
	Entity                      string
	EntityAttributesPreposition string
	EntityAttributes            []string
	ContainerEntityPreposition  string
	ContainerEntityArticle      string
	ContainerEntity             string
}

// StoryFeature represents the StoryFeature grammar rule.
type StoryFeature struct {
	Node
	Verb                        string
	EntityArticle               string
	Entity                      string
	EntityAttributesPreposition string
	EntityAttributes            []string
	ContainerEntityPreposition  string
	ContainerEntityArticle      string
	ContainerEntity             string
}

// StoryValuation represents the StoryValuation grammar rule.
type StoryValuation struct {
	Node
	PromotedValues []string
	HarmedValues   []string
}

// UserActivityDefaultVerb represents the UserActivityDefaultVerb grammar rule.
type UserActivityDefaultVerb string

// SculptorModule represents the SculptorModule grammar rule.
type SculptorModule struct {
	Node
	Doc           string
	Name          string
	External      bool
	BasePackage   string
	Hint          string
	Services      []*Service
	Resources     []*Resource
	Consumers     []*Consumer
	DomainObjects []SimpleDomainObject
	Aggregates    []*Aggregate
}

// Stakeholders represents the Stakeholders grammar rule.
type Stakeholders struct {
	Node
	Contexts     []Ref
	Stakeholders []AbstractStakeholder
}

// AbstractStakeholder represents the AbstractStakeholder grammar rule.
type AbstractStakeholder interface{ abstractStakeholder() *Node }

// StakeholderGroup represents the StakeholderGroup grammar rule.
type StakeholderGroup struct {
	Node
	Name         string
	Stakeholders []*Stakeholder
}

// Stakeholder represents the Stakeholder grammar rule.
type Stakeholder struct {
	Node
	Name        string
	Influence   INFLUENCE
	Interest    INTEREST
	Description string
}

// ValueRegister represents the ValueRegister grammar rule.
type ValueRegister struct {
	Node
	Name            string
	Context         Ref
	ValueClusters   []*ValueCluster
	Values          []*Value
	ValueEpics      []*ValueEpic
	ValueNarratives []*ValueNarrative
	ValueWeightings []*ValueWeigthing
}

// ValueCluster represents the ValueCluster grammar rule.
type ValueCluster struct {
	Node
	Name           string
	CoreValue7000  CoreValue
	CoreValue      string
	Demonstrators  []string
	RelatedValues  []string
	OpposingValues []string
	Values         []*Value
	Elicitations   []*ValueElicitation
}

// Value represents the Value grammar rule.
type Value struct {
	Node
	Name           string
	CoreValue      bool
	Demonstrators  []string
	RelatedValues  []string
	OpposingValues []string
	Elicitations   []*ValueElicitation
}

// ValueElicitation represents the ValueElicitation grammar rule.
type ValueElicitation struct {
	Node
	Stakeholder  Ref
	Priority     PRIORITY
	Impact       IMPACT
	Consequences []*Consequence
}

// ValueEpic represents the ValueEpic grammar rule.
type ValueEpic struct {
	Node
	Name           string
	Stakeholder    Ref
	Value          string
	RealizedValues []string
	ReducedValues  []string
}

// ValueNarrative represents the ValueNarrative grammar rule.
type ValueNarrative struct {
	Node
	Name                 string
	Feature              string
	PromotedValues       string
	HarmedValues         string
	PreAndPostConditions string
}

// ValueWeigthing represents the ValueWeigthing grammar rule.
type ValueWeigthing struct {
	Node
	Name        string
	Stakeholder Ref
	Value1      string
	Value2      string
	Benefits    string
	Harms       string
}

// Consequence represents the Consequence grammar rule.
type Consequence struct {
	Node
	Type        string
	Consequence string
	Action      *Action
}

// Action represents the Action grammar rule.
type Action struct {
	Node
	Action string
	Type   string
}

// TacticDDDModel represents the TacticDDDModel grammar rule.
type TacticDDDModel struct {
	Node
	App *TacticDDDApplication
}

// TacticDDDApplication represents the TacticDDDApplication grammar rule.
type TacticDDDApplication struct {
	Node
	Doc           string
	Name          string
	BasePackage   string
	Services      []*Service
	Resources     []*Resource
	Consumers     []*Consumer
	DomainObjects []SimpleDomainObject
}

// Service represents the Service grammar rule.
type Service struct {
	Node
	Comment      string
	Doc          string
	Name         string
	GapClass     bool
	NoGapClass   bool
	Hint         string
	WebService   bool
	Subscribe    *Subscribe
	Dependencies []*Dependency
	Operations   []*ServiceOperation
	Associations []*Association
}

// Resource represents the Resource grammar rule.
type Resource struct {
	Node
	Doc          string
	Name         string
	GapClass     bool
	NoGapClass   bool
	Hint         string
	Scaffold     bool
	Path         string
	Dependencies []*ServiceDependency
	Operations   []*ResourceOperation
}

// Consumer represents the Consumer grammar rule.
type Consumer struct {
	Node
	Doc          string
	Name         string
	Hint         string
	Dependencies []*Dependency
	MessageRoot  Ref
	Channel      string
	Subscribe    *Subscribe
}

// Subscribe represents the Subscribe grammar rule.
type Subscribe struct {
	Node
	Topic    string
	EventBus string
}

// Publish represents the Publish grammar rule.
type Publish struct {
	Node
	EventType Ref
	Topic     string
	EventBus  string
}

// Event represents the Event grammar rule.
type Event interface{ event() *Node }

// DomainObjectTypedElement represents the DomainObjectTypedElement grammar rule.
type DomainObjectTypedElement interface{ domainObjectTypedElement() *Node }

// ServiceOperation represents the ServiceOperation grammar rule.
type ServiceOperation struct {
	Node
	Doc               string
	Visibility        Visibility
	ReturnType        *ComplexType
	Name              string
	Parameters        []*Parameter
	Throws            string
	Hint              string
	Publish           *Publish
	DelegateHolder    *ServiceOperationDelegate
	ReadOnlyOperation bool
	WriteOperation    bool
	StateTransition   *StateTransition
}

// StateTransition represents the StateTransition grammar rule.
type StateTransition struct {
	Node
	From   []Ref
	Target StateTransitionTarget
}

// StateTransitionTarget represents the StateTransitionTarget grammar rule.
type StateTransitionTarget interface{ stateTransitionTarget() *Node }

// SingleStateTransitionTarget represents the SingleStateTransitionTarget grammar rule.
type SingleStateTransitionTarget struct {
	Node
	To []*TargetState
}

// ExclusiveAlternativeStateTransitionTarget represents the ExclusiveAlternativeStateTransitionTarget grammar rule.
type ExclusiveAlternativeStateTransitionTarget struct {
	Node
	To []*TargetState
}

// TargetState represents the TargetState grammar rule.
type TargetState struct {
	Node
	Value    Ref
	EndState bool
}

// ServiceOperationDelegate represents the ServiceOperationDelegate grammar rule.
type ServiceOperationDelegate struct {
	Node
	Delegate          Ref
	DelegateOperation Ref
}

// ServiceRepositoryOption represents the ServiceRepositoryOption grammar rule.
type ServiceRepositoryOption interface{ serviceRepositoryOption() *Node }

// ServiceRepositoryOperationOption represents the ServiceRepositoryOperationOption grammar rule.
type ServiceRepositoryOperationOption interface{ serviceRepositoryOperationOption() *Node }

// ResourceOperation represents the ResourceOperation grammar rule.
type ResourceOperation struct {
	Node
	Doc            string
	Visibility     Visibility
	ReturnType     *ComplexType
	Name           string
	Parameters     []*Parameter
	Throws         string
	Hint           string
	HTTPMethod     HTTPMethod
	Path           string
	ReturnString   string
	DelegateHolder *ResourceOperationDelegate
}

// ResourceOperationDelegate represents the ResourceOperationDelegate grammar rule.
type ResourceOperationDelegate struct {
	Node
	Delegate          Ref
	DelegateOperation Ref
}

// RepositoryOperation represents the RepositoryOperation grammar rule.
type RepositoryOperation struct {
	Node
	Doc                    string
	Visibility             Visibility
	ReturnType             *ComplexType
	Name                   string
	Parameters             []*Parameter
	Throws                 string
	Hint                   string
	Cache                  bool
	GapOperation           bool
	NoGapOperation         bool
	Query                  string
	Condition              string
	Select                 string
	GroupBy                string
	OrderBy                string
	Construct              bool
	Build                  bool
	Map                    bool
	Publish                *Publish
	DelegateToAccessObject bool
	AccessObjectName       string
}

// Parameter represents the Parameter grammar rule.
type Parameter struct {
	Node
	Doc           string
	ParameterType *ComplexType
	Name          string
}

// ComplexType represents the ComplexType grammar rule.
type ComplexType struct {
	Node
	Type                   string
	DomainObjectType       Ref
	CollectionType         CollectionType
	MapCollectionType      string
	MapKeyType             string
	MapKeyDomainObjectType Ref
}

// SimpleDomainObject represents the SimpleDomainObject grammar rule.
type SimpleDomainObject interface{ simpleDomainObject() *Node }

// DomainObject represents the DomainObject grammar rule.
type DomainObject interface{ domainObject() *Node }

// Entity represents the Entity grammar rule.
type Entity struct {
	Node
	Comment              string
	Doc                  string
	Abstract             bool
	Name                 string
	Extends              Ref
	ExtendsName          string
	Traits               []Ref
	Package              string
	NotOptimisticLocking bool
	NotAuditable         bool
	Cache                bool
	GapClass             bool
	NoGapClass           bool
	Scaffold             bool
	Hint                 string
	DatabaseTable        string
	DiscriminatorValue   string
	DiscriminatorColumn  string
	DiscriminatorType    DiscriminatorType
	DiscriminatorLength  string
	InheritanceType      InheritanceType
	Validate             string
	AggregateRoot        bool
	BelongsTo            Ref
	Attributes           []*Attribute
	References           []*Reference
	Operations           []*DomainObjectOperation
	Associations         []*Association
	Repository           *Repository
}

// ValueObject represents the ValueObject grammar rule.
type ValueObject struct {
	Node
	Comment              string
	Doc                  string
	Abstract             bool
	Name                 string
	Extends              Ref
	ExtendsName          string
	Traits               []Ref
	Package              string
	NotOptimisticLocking bool
	NotImmutable         bool
	Cache                bool
	GapClass             bool
	NoGapClass           bool
	Scaffold             bool
	Hint                 string
	DatabaseTable        string
	DiscriminatorValue   string
	DiscriminatorColumn  string
	DiscriminatorType    DiscriminatorType
	DiscriminatorLength  string
	InheritanceType      InheritanceType
	Validate             string
	Persistent           bool
	NotPersistent        bool
	AggregateRoot        bool
	BelongsTo            Ref
	Attributes           []*Attribute
	References           []*Reference
	Operations           []*DomainObjectOperation
	Associations         []*Association
	Repository           *Repository
}

// DomainEvent represents the DomainEvent grammar rule.
type DomainEvent struct {
	Node
	Comment             string
	Doc                 string
	Abstract            bool
	Name                string
	Extends             Ref
	ExtendsName         string
	Traits              []Ref
	Package             string
	Cache               bool
	GapClass            bool
	NoGapClass          bool
	Scaffold            bool
	Hint                string
	DatabaseTable       string
	DiscriminatorValue  string
	DiscriminatorColumn string
	DiscriminatorType   DiscriminatorType
	DiscriminatorLength string
	InheritanceType     InheritanceType
	Validate            string
	Persistent          bool
	AggregateRoot       bool
	BelongsTo           Ref
	Attributes          []*Attribute
	References          []*Reference
	Operations          []*DomainObjectOperation
	Associations        []*Association
	Repository          *Repository
}

// CommandEvent represents the CommandEvent grammar rule.
type CommandEvent struct {
	Node
	Comment             string
	Doc                 string
	Abstract            bool
	Name                string
	Extends             Ref
	ExtendsName         string
	Traits              []Ref
	Package             string
	Cache               bool
	GapClass            bool
	NoGapClass          bool
	Scaffold            bool
	Hint                string
	DatabaseTable       string
	DiscriminatorValue  string
	DiscriminatorColumn string
	DiscriminatorType   DiscriminatorType
	DiscriminatorLength string
	InheritanceType     InheritanceType
	Validate            string
	Persistent          bool
	AggregateRoot       bool
	BelongsTo           Ref
	Attributes          []*Attribute
	References          []*Reference
	Operations          []*DomainObjectOperation
	Associations        []*Association
	Repository          *Repository
}

// Trait represents the Trait grammar rule.
type Trait struct {
	Node
	Doc          string
	Name         string
	Package      string
	Hint         string
	Attributes   []*Attribute
	References   []*Reference
	Operations   []*DomainObjectOperation
	Associations []*Association
}

// DomainObjectOperation represents the DomainObjectOperation grammar rule.
type DomainObjectOperation struct {
	Node
	Doc               string
	Abstract          bool
	Visibility        Visibility
	ReturnType        *ComplexType
	Name              string
	Parameters        []*Parameter
	Throws            string
	Hint              string
	ReadOnlyOperation bool
	WriteOperation    bool
	StateTransition   *StateTransition
}

// DataTransferObject represents the DataTransferObject grammar rule.
type DataTransferObject struct {
	Node
	Comment     string
	Doc         string
	Abstract    bool
	Name        string
	Extends     Ref
	ExtendsName string
	Package     string
	GapClass    bool
	NoGapClass  bool
	Hint        string
	Validate    string
	Attributes  []*DtoAttribute
	References  []*DtoReference
}

// BasicType represents the BasicType grammar rule.
type BasicType struct {
	Node
	Doc          string
	Name         string
	Traits       []Ref
	Package      string
	NotImmutable bool
	GapClass     bool
	NoGapClass   bool
	Hint         string
	Attributes   []*Attribute
	References   []*Reference
	Operations   []*DomainObjectOperation
	Associations []*Association
}

// Attribute represents the Attribute grammar rule.
type Attribute struct {
	Node
	Doc                     string
	Visibility              Visibility
	CollectionType          CollectionType
	Type                    string
	Name                    string
	Key                     bool
	NotChangeable           bool
	Required                bool
	Unique                  bool
	Nullable                bool
	NullableMessage         string
	Index                   bool
	AssertFalse             bool
	AssertFalseMessage      string
	AssertTrue              bool
	AssertTrueMessage       string
	Hint                    string
	CreditCardNumber        bool
	CreditCardNumberMessage string
	Digits                  string
	Email                   bool
	EmailMessage            string
	Future                  bool
	FutureMessage           string
	Past                    bool
	PastMessage             string
	Max                     string
	Min                     string
	DecimalMax              string
	DecimalMin              string
	NotEmpty                bool
	NotEmptyMessage         string
	NotBlank                bool
	NotBlankMessage         string
	Pattern                 string
	Range                   string
	Size                    string
	Length                  string
	ScriptAssert            string
	URL                     string
	Validate                string
	Transient               bool
	DatabaseColumn          string
	DatabaseType            string
}

// Reference represents the Reference grammar rule.
type Reference struct {
	Node
	Doc                string
	Visibility         Visibility
	CollectionType     CollectionType
	DomainObjectType   Ref
	Name               string
	Key                bool
	NotChangeable      bool
	Required           bool
	Nullable           bool
	NullableMessage    string
	Hint               string
	Cascade            string
	Fetch              string
	Cache              bool
	Inverse            bool
	DatabaseColumn     string
	DatabaseJoinTable  string
	DatabaseJoinColumn string
	NotEmpty           bool
	NotEmptyMessage    string
	Size               string
	Valid              bool
	ValidMessage       string
	Validate           string
	Transient          bool
	OrderBy            string
	OrderColumn        bool
	OrderColumnName    string
	OppositeHolder     *OppositeHolder
	AssociationLabel   string
}

// Association represents the Association grammar rule.
type Association struct {
	Node
	Doc              string
	Description      string
	DomainObjectType Ref
}

// DtoAttribute represents the DtoAttribute grammar rule.
type DtoAttribute struct {
	Node
	Doc                     string
	Visibility              Visibility
	CollectionType          CollectionType
	Type                    string
	Name                    string
	Key                     bool
	NotChangeable           bool
	Required                bool
	Nullable                bool
	NullableMessage         string
	Transient               bool
	AssertFalse             bool
	AssertFalseMessage      string
	AssertTrue              bool
	AssertTrueMessage       string
	CreditCardNumber        bool
	CreditCardNumberMessage string
	Digits                  string
	Email                   bool
	EmailMessage            string
	Future                  bool
	FutureMessage           string
	Past                    bool
	PastMessage             string
	Max                     string
	Min                     string
	DecimalMax              string
	DecimalMin              string
	NotEmpty                bool
	NotEmptyMessage         string
	NotBlank                bool
	NotBlankMessage         string
	Pattern                 string
	Range                   string
	Size                    string
	Length                  string
	ScriptAssert            string
	URL                     string
	Validate                string
	Hint                    string
}

// DtoReference represents the DtoReference grammar rule.
type DtoReference struct {
	Node
	Doc              string
	Visibility       Visibility
	CollectionType   CollectionType
	DomainObjectType Ref
	Name             string
	Key              bool
	NotChangeable    bool
	Required         bool
	Nullable         bool
	NullableMessage  string
	Transient        bool
	NotEmpty         bool
	NotEmptyMessage  string
	Size             string
	Valid            bool
	ValidMessage     string
	Validate         string
	Hint             string
}

// OppositeHolder represents the OppositeHolder grammar rule.
type OppositeHolder struct {
	Node
	Opposite Ref
}

// Repository represents the Repository grammar rule.
type Repository struct {
	Node
	Doc          string
	Name         string
	GapClass     bool
	NoGapClass   bool
	Hint         string
	Subscribe    *Subscribe
	Dependencies []*Dependency
	Operations   []*RepositoryOperation
}

// ServiceDependency represents the ServiceDependency grammar rule.
type ServiceDependency struct {
	Node
	Dependency Ref
}

// Dependency represents the Dependency grammar rule.
type Dependency struct {
	Node
	Dependency Ref
	Name       string
}

// Enum represents the Enum grammar rule.
type Enum struct {
	Node
	Doc                       string
	Name                      string
	Package                   string
	Hint                      string
	DefinesAggregateLifecycle bool
	Ordinal                   bool
	Attributes                []*EnumAttribute
	Values                    []*EnumValue
}

// EnumAttribute represents the EnumAttribute grammar rule.
type EnumAttribute struct {
	Node
	Doc  string
	Type string
	Name string
	Key  bool
}

// EnumValue represents the EnumValue grammar rule.
type EnumValue struct {
	Node
	Doc        string
	Name       string
	Parameters []*EnumParameter
}

// EnumParameter represents the EnumParameter grammar rule.
type EnumParameter struct {
	Node
	Value        string
	IntegerValue int
}

// AnyProperty represents the AnyProperty grammar rule.
type AnyProperty interface{ anyProperty() *Node }

// Property represents the Property grammar rule.
type Property interface{ property() *Node }

// DtoProperty represents the DtoProperty grammar rule.
type DtoProperty interface{ dtoProperty() *Node }

// Type represents the Type grammar rule.
type Type string

// JavaIdentifier represents the JavaIdentifier grammar rule.
type JavaIdentifier string

// ChannelIdentifier represents the ChannelIdentifier grammar rule.
type ChannelIdentifier string

// ThrowsIdentifier represents the ThrowsIdentifier grammar rule.
type ThrowsIdentifier string

// UpstreamRole represents the UpstreamRole grammar rule.
type UpstreamRole string

// DownstreamRole represents the DownstreamRole grammar rule.
type DownstreamRole string

// ContextMapState represents the ContextMapState grammar rule.
type ContextMapState string

// ContextMapType represents the ContextMapType grammar rule.
type ContextMapType string

// BoundedContextType represents the BoundedContextType grammar rule.
type BoundedContextType string

// SubDomainType represents the SubDomainType grammar rule.
type SubDomainType string

// DownstreamGovernanceRights represents the DownstreamGovernanceRights grammar rule.
type DownstreamGovernanceRights string

// KnowledgeLevel represents the KnowledgeLevel grammar rule.
type KnowledgeLevel string

// Volatility represents the Volatility grammar rule.
type Volatility string

// Criticality represents the Criticality grammar rule.
type Criticality string

// Similarity represents the Similarity grammar rule.
type Similarity string

// Evolution represents the Evolution grammar rule.
type Evolution string

// INFLUENCE represents the INFLUENCE grammar rule.
type INFLUENCE string

// INTEREST represents the INTEREST grammar rule.
type INTEREST string

// PRIORITY represents the PRIORITY grammar rule.
type PRIORITY string

// IMPACT represents the IMPACT grammar rule.
type IMPACT string

// CoreValue represents the CoreValue grammar rule.
type CoreValue string

// HTTPMethod represents the HTTPMethod grammar rule.
type HTTPMethod string

// InheritanceType represents the InheritanceType grammar rule.
type InheritanceType string

// DiscriminatorType represents the DiscriminatorType grammar rule.
type DiscriminatorType string

// CollectionType represents the CollectionType grammar rule.
type CollectionType string

// Visibility represents the Visibility grammar rule.
type Visibility string

func (v *Domain) domainPart() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Subdomain) domainPart() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Partnership) relationship() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *SharedKernel) relationship() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *UpstreamDownstreamRelationship) relationship() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *CustomerSupplierRelationship) relationship() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Partnership) symmetricRelationship() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *SharedKernel) symmetricRelationship() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DomainEventProductionStep) flowStep() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *CommandInvokationStep) flowStep() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *SingleCommandInvokation) eitherCommandOrOperationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ConcurrentCommandInvokation) eitherCommandOrOperationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ExclusiveAlternativeCommandInvokation) eitherCommandOrOperationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *InclusiveAlternativeCommandInvokation) eitherCommandOrOperationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *SingleOperationInvokation) eitherCommandOrOperationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ConcurrentOperationInvokation) eitherCommandOrOperationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ExclusiveAlternativeOperationInvokation) eitherCommandOrOperationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *InclusiveAlternativeOperationInvokation) eitherCommandOrOperationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *SingleCommandInvokation) commandInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ConcurrentCommandInvokation) commandInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ExclusiveAlternativeCommandInvokation) commandInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *InclusiveAlternativeCommandInvokation) commandInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *SingleOperationInvokation) operationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ConcurrentOperationInvokation) operationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ExclusiveAlternativeOperationInvokation) operationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *InclusiveAlternativeOperationInvokation) operationInvokation() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *SingleEventProduction) eventProduction() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *MultipleEventProduction) eventProduction() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ExclusiveAlternativeEventProduction) eventProduction() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *InclusiveAlternativeEventProduction) eventProduction() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *UseCase) userRequirement() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *UserStory) userRequirement() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *NormalFeature) feature() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *StoryFeature) feature() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *StakeholderGroup) abstractStakeholder() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Stakeholder) abstractStakeholder() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DomainEvent) event() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *CommandEvent) event() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ServiceOperation) domainObjectTypedElement() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *RepositoryOperation) domainObjectTypedElement() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DomainObjectOperation) domainObjectTypedElement() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Parameter) domainObjectTypedElement() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *SingleStateTransitionTarget) stateTransitionTarget() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ExclusiveAlternativeStateTransitionTarget) stateTransitionTarget() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Repository) serviceRepositoryOption() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Service) serviceRepositoryOption() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *RepositoryOperation) serviceRepositoryOperationOption() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ServiceOperation) serviceRepositoryOperationOption() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *BasicType) simpleDomainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Enum) simpleDomainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Entity) simpleDomainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ValueObject) simpleDomainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DomainEvent) simpleDomainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *CommandEvent) simpleDomainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DataTransferObject) simpleDomainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Trait) simpleDomainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Entity) domainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *ValueObject) domainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DomainEvent) domainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *CommandEvent) domainObject() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Attribute) anyProperty() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Reference) anyProperty() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DtoAttribute) anyProperty() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DtoReference) anyProperty() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Attribute) property() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *Reference) property() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DtoAttribute) dtoProperty() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}

func (v *DtoReference) dtoProperty() *Node {
	if v == nil {
		return nil
	}
	return &v.Node
}
