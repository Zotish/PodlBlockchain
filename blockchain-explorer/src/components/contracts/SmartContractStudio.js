// import React, { useState } from "react";
// import ContractDeploy from "./    DeployContract";
// import ContractList from "./    CallContract";
// import ContractABI from "./    ContractABI";
// import ContractManager from "./ContractManager";


// const SmartContractStudio = ({ walletAddress }) => {
//   const [selectedFn, setSelectedFn] = useState(null);

//   return (
//     <div className="remix-ui">
//       <div className="sidebar">
//         <ContractDeploy walletAddress={walletAddress} />
//         <ContractList onSelect={(addr) => setSelectedFn({ address: addr })} />
//       </div>

//       <div className="main-panel">
//         <ContractABI setSelectedFn={setSelectedFn} />
//         <ContractManager selectedFn={selectedFn} walletAddress={walletAddress} />
//       </div>
//     </div>
//   );
// };

// export default SmartContractStudio;



import React, { useState } from "react";
import ContractManager from "./ContractManager";
import ContractList from "./ContractList";

const SmartContractStudio = ({ walletAddress }) => {
  const [selectedAddress, setSelectedAddress] = useState(null);

  return (
    <div className="contract-studio">
      <div className="sidebar">
        <ContractList onSelect={(addr) => setSelectedAddress(addr)} />
      </div>

      <div className="main-panel">
        <ContractManager address={walletAddress} />
      </div>
    </div>
  );
};

export default SmartContractStudio;



