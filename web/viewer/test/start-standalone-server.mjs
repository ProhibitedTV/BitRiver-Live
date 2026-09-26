import path from "node:path";
import { pathToFileURL } from "node:url";

await import("./prepare-standalone-server.mjs");

const serverPath = path.join(process.cwd(), ".next", "standalone", "server.js");
await import(pathToFileURL(serverPath).href);
