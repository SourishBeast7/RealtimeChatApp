import { BrowserRouter as Router, Routes, Route, Link } from "react-router-dom";

import Home from "./pages/Home";
import Login from "./pages/Login";
import Signup from "./pages/Signup";

//Image imported here

import myImage from '/logo.png?url'

//CSS imported Here

import "./App.css";
import Chat from "./pages/Chat";

const App = () => {
  return (
    <Router>
      <div className="overflow-hidden absolute">
        <nav className="navbar p-5 bg-amber-500 flex gap-14 text-xl sticky top-0 min-w-screen z-50 justify-between items-center">
        <Link to="/"><img src={myImage} height={"50px"} width={"70px"}/></Link>
        <div className="flex gap-14 text-xl">
          <Link to="/">Home</Link>
          <Link to="/signup">Signup</Link>
          <Link to="/login">Login</Link>
        </div>
      </nav>
      </div>

      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/signup" element={<Signup />} />
        <Route path="/login" element={<Login />} />
        <Route path="/app/chat" element={<Chat/>}/>
      </Routes>
    </Router>
  );
};

export default App;
