// import React, { useState } from "react";
// import DeployContract from "./    DeployContract";
// import CallContract from "./    CallContract";
// import ContractABI from "./    ContractABI";


// const ContractManager = ({ address, privateKey }) => {
//   const [active, setActive] = useState("deploy");

//   return (
//     <div>
//       <h3>Smart Contract Manager</h3>

//       <div className="wallet-tabs">
//         <button 
//           className={`tab ${active === "deploy" ? "active" : ""}`} 
//           onClick={() => setActive("deploy")}
//         >
//           Deploy
//         </button>

//         <button 
//           className={`tab ${active === "call" ? "active" : ""}`} 
//           onClick={() => setActive("call")}
//         >
//           Call
//         </button>

//         <button 
//           className={`tab ${active === "abi" ? "active" : ""}`} 
//           onClick={() => setActive("abi")}
//         >
//           ABI
//         </button>
//       </div>

//       {active === "deploy" && <DeployContract wallet={address} />}
//       {active === "call" && <CallContract wallet={address} />}
//       {active === "abi" && <ContractABI />}
//     </div>
//   );
// };

// export default ContractManager;



import React, { useState } from "react";
import DeployContract from "./ DeployContract";
import CallContract from "./CallContract";
import ContractABI from "./    ContractABI";

const ContractManager = ({ address, privateKey }) => {
  const [active, setActive] = useState("deploy");

  return (
    <div className="contract-manager">
      <h3>Smart Contract Manager</h3>

      <div className="wallet-tabs">
        <button
          className={`tab ${active === "deploy" ? "active" : ""}`}
          onClick={() => setActive("deploy")}
        >
          Deploy
        </button>

        <button
          className={`tab ${active === "call" ? "active" : ""}`}
          onClick={() => setActive("call")}
        >
          Call
        </button>

        <button
          className={`tab ${active === "abi" ? "active" : ""}`}
          onClick={() => setActive("abi")}
        >
          ABI
        </button>
      </div>

      {active === "deploy" && <DeployContract walletAddress={address} privateKey={privateKey} />}
      {active === "call" && <CallContract walletAddress={address} privateKey={privateKey} />}
      {active === "abi" && <ContractABI />}
    </div>
  );
};

export default ContractManager;

