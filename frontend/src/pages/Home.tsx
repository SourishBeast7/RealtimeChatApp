import { useNavigate } from "react-router-dom";
const Home = () => {
  const navigator = useNavigate();
  return (
    <div className="h-fit relative bg-amber-300 max-w-screen">
      <div className="slider min-h-170 flex items-center justify-around flex-col gap-4">
        <h1 className="heading text-6xl text-center">Stick People Together</h1>

        <button
          onClick={() => {
            navigator("/signup");
          }}
          className="cursor-pointer text-center text-xl p-1 px-3.5 bg-emerald-600 rounded-md border-2 h-13 border-black "
        >
          Get Started
        </button>
        {/* <span>He</span> */}
      </div>
    </div>
  );
};

export default Home;
