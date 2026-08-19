package gotopo

type AssignmentPriority string

const (
	PriorityLow    AssignmentPriority = "LOW"
	PriorityMedium AssignmentPriority = "MEDIUM"
	PriorityHigh   AssignmentPriority = "HIGH"
)

type AssignmentStatus string

const (
	AssignmentDraft      AssignmentStatus = "DRAFT"
	AssignmentPrepared   AssignmentStatus = "PREPARED"
	AssignmentInProgress AssignmentStatus = "INPROGRESS"
	AssignmentCompleted  AssignmentStatus = "COMPLETED"
)

type ResourceType string

const (
	ResourceGround   ResourceType = "GROUND"
	ResourceGround1  ResourceType = "GROUND_1"
	ResourceGround2  ResourceType = "GROUND_2"
	ResourceGround3  ResourceType = "GROUND_3"
	ResourceDog      ResourceType = "DOG"
	ResourceDogTrail ResourceType = "DOG_TRAIL"
	ResourceDogArea  ResourceType = "DOG_AREA"
	ResourceDogHRD   ResourceType = "DOG_HRD"
	ResourceOHV      ResourceType = "OHV"
	ResourceBike     ResourceType = "BIKE"
	ResourceWater    ResourceType = "WATER"
	ResourceMounted  ResourceType = "MOUNTED"
	ResourceAir      ResourceType = "AIR"
)
