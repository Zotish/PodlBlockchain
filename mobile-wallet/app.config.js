const pkg = require("./package.json");

module.exports = ({ config }) => ({
  ...config,
  name: "LQD Mobile Wallet",
  slug: "lqd-mobile-wallet",
  version: pkg.version,
  scheme: "lqdwallet",
  orientation: "portrait",
  userInterfaceStyle: "dark",
  android: {
    package: "com.zotish.lqdmobilewallet",
    versionCode: 2,
  },
  ios: {
    bundleIdentifier: "com.zotish.lqdmobilewallet",
    buildNumber: "2",
  },
  plugins: [
    "expo-asset",
    "expo-secure-store",
    "expo-camera",
    "expo-local-authentication",
  ],
});
