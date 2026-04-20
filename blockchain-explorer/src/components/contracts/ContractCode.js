import React, { useEffect, useState } from "react";
import { API_BASE, apiUrl } from "../../utils/api";

const API = API_BASE;

export default function ContractCode({ address }) {
  const [info, setInfo] = useState(null);

  useEffect(() => {
    fetch(apiUrl(API, `/contract/code?address=${address}`))
      .then((r) => r.json())
      .then((d) => setInfo(d));
  }, [address]);

  if (!info) return <p>Loading contract code...</p>;
  if (info.error) return <p>{info.error}</p>;

  return (
    <div className="contract-panel">
      <h3>📄 Contract Code</h3>

      <p><strong>Type:</strong> {info.type}</p>

      {info.source && (
        <>
          <h4>Source Code</h4>
          <pre className="code-block">{info.source}</pre>
        </>
      )}

      {info.bytecode && (
        <>
          <h4>Bytecode</h4>
          <pre className="code-block">{info.bytecode}</pre>
        </>
      )}

      {info.pluginPath && (
        <>
          <h4>Plugin File</h4>
          <pre className="code-block">{info.pluginPath}</pre>
        </>
      )}
    </div>
  );
}
