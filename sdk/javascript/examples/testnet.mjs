import { PoDLClient } from "../src/index.js";

const client = new PoDLClient(process.env.PODL_URL || "http://127.0.0.1:6500");
const [protocol, readiness] = await Promise.all([client.protocolStatus(), client.mainnetReadiness()]);
console.log(JSON.stringify({ protocol, readiness }, null, 2));
