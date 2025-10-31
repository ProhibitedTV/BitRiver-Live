declare module "ovenplayer" {
  const OvenPlayer: {
    create: (containerId: string, options: unknown) => { remove?: () => void };
  };
  export default OvenPlayer;
}
