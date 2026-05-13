import { RadialAvatar } from "./radial-avatar";

const Header = () => {
  return (
    <div className="flex debug justify-between items-center">
      <div className="ring-2 ring-lime-950 rounded-lg p-[2px] hover:ring-lime-900 transition-all duration-300 cursor-pointer group">
        <div className="rounded-md p-1 px-1.5 bg-lime-950 transition-all duration-300">
          <p className="text-white font-medium">Orion</p>
        </div>
      </div>
      <RadialAvatar seed="casprin-eSs" size="sm" />
    </div>
  );
};

export const Layout = ({ children }: { children: React.ReactNode }) => {
  return (
    <div className="flex flex-col min-h-screen max-w-4xl mx-auto py-10">
      <Header />
      {children}
    </div>
  );
};
