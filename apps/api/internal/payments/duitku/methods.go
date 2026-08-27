package duitku

// Payment method codes Duitku's classic API expects in the
// paymentMethod field. QRIS and BRIVA are confirmed from this project's
// own prior integration experience; the rest come from Duitku's public
// documentation and haven't been exercised here — confirm against your
// live merchant dashboard before relying on one that isn't marked
// "confirmed" below, since gateways do add/retire codes over time.
const (
	MethodQRIS      = "SP" // standard QRIS — confirmed from prior integration experience
	MethodQRISNobu  = "QN" // QRIS via Bank Nobu specifically — documented, not confirmed here
	MethodBRIVA     = "BR" // BRI Virtual Account — confirmed from prior integration experience
	MethodBCAVA     = "BC" // BCA Virtual Account — documented, not confirmed here
	MethodMandiriVA = "M2" // Mandiri Virtual Account — documented, not confirmed here
	MethodPermataVA = "BT" // Permata Virtual Account — documented, not confirmed here
	MethodCIMBVA    = "B1" // CIMB Niaga Virtual Account — documented, not confirmed here
)
