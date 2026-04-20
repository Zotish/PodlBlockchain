import React, { useEffect, useState } from "react";
import { API_BASE, apiUrl } from "../../utils/api";

const API = API_BASE;

export default function ContractEvents({ address }) {
  const [events, setEvents] = useState([]);

  useEffect(() => {
    fetch(apiUrl(API, `/contract/events?address=${address}`))
      .then((r) => r.json())
      .then((d) => setEvents(d));
  }, [address]);

  return (
    <div>
      <h3>📢 Events</h3>
      <pre>{JSON.stringify(events, null, 2)}</pre>
    </div>
  );
}
