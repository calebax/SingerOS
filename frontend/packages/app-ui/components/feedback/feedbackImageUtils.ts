const ACCEPTED_IMAGE_TYPES = new Set(["image/jpeg", "image/jpg", "image/png", "image/webp"]);

export const FEEDBACK_IMAGE_ACCEPT = "image/jpeg,image/jpg,image/png,image/webp";
export const FEEDBACK_MAX_IMAGES = 9;

export function isAcceptedFeedbackImage(file: File): boolean {
	const type = file.type.toLowerCase();
	if (ACCEPTED_IMAGE_TYPES.has(type)) {
		return true;
	}
	return /\.(jpe?g|png|webp)$/i.test(file.name);
}
