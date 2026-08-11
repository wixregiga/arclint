import path from "node:path";
import _ from "lodash";
import { validate } from "./validate";

export const user = { name: path.basename("u"), check: () => validate(_.identity("u")) };
