import type { ProjectRequest } from "../../../shared/transport/index";

export interface ProjectValues {
  name: string;
  description: string;
}

/**
 * The project edit a form makes: only the properties whose trimmed text
 * differs from what is recorded, so an untouched one stays as it is; null
 * when nothing differs.
 */
export function projectRequestFrom(recorded: ProjectValues, values: ProjectValues): ProjectRequest | null {
  const request: ProjectRequest = {};
  const name = values.name.trim();
  const description = values.description.trim();
  if (name !== recorded.name) request.name = name;
  if (description !== recorded.description) request.description = description;
  return Object.keys(request).length === 0 ? null : request;
}
