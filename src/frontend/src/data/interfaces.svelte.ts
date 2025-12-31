import { type Interfaces } from "../types";
import { fetcher } from "../utils/fetcher";

export const INTERFACES = $state<Interfaces["interfaces"]>(
  await fetcher
    .get<Interfaces>("/system/interfaces")
    .then((data) => data.interfaces)
    .catch((error) => {
      console.error(error);
      return [];
    })
);
