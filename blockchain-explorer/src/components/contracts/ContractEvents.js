import React, { useEffect, useState } from "react";

const API = "http://127.0.0.1:9000";

export default function ContractEvents({ address }) {
  const [events, setEvents] = useState([]);

  useEffect(() => {
    fetch(`${API}/contract/events?address=${address}`)
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
