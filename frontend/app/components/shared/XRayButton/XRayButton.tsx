import React, { useEffect, useState } from 'react';
import stl from './xrayButton.module.css';
import cn from 'classnames';
import { Popup } from 'UI';
import { Tooltip } from 'react-tippy';
import { Controls as Player } from 'Player';

interface Props {
  onClick?: () => void;
  isActive?: boolean;
}
function XRayButton(props: Props) {
  const { isActive } = props;
  const [showGuide, setShowGuide] = useState(!localStorage.getItem('featureViewed'));
  useEffect(() => {
    if (!showGuide) {
      return;
    }
    Player.pause();
  }, []);

  const onClick = () => {
    setShowGuide(false);
    localStorage.setItem('featureViewed', 'true');
    props.onClick();
  };
  return (
    <>
      {showGuide && (
        <div
          onClick={() => {
            setShowGuide(false);
            localStorage.setItem('featureViewed', 'true');
          }}
          className="bg-gray-darkest fixed inset-0 z-10 w-full h-screen"
          style={{ zIndex: 9999, opacity: '0.7' }}
        ></div>
      )}
      <div className="relative">
        {showGuide ? (
          <GuidePopup>
            <button
              className={cn(stl.wrapper, { [stl.default]: !isActive, [stl.active]: isActive })}
              onClick={onClick}
              style={{ zIndex: 99999, position: 'relative' }}
            >
              <span className="z-1">X-RAY</span>
            </button>

            <div
              className="absolute bg-white top-0 left-0 z-0"
              style={{
                zIndex: 99998,
                width: '100px',
                height: '50px',
                borderRadius: '30px',
                margin: '-10px -16px',
              }}
            ></div>
          </GuidePopup>
        ) : (
          <Popup content="Get a quick overview on the issues in this session." disabled={isActive}>
            <button
              className={cn(stl.wrapper, { [stl.default]: !isActive, [stl.active]: isActive })}
              onClick={onClick}
            >
              <span className="z-1">X-RAY</span>
            </button>
          </Popup>
        )}
      </div>
    </>
  );
}

export default XRayButton;

function GuidePopup({ children }: any) {
  return (
    <Tooltip
      html={
        <div>
          <div className="font-bold">
            Introducing <span className={stl.text}>X-Ray</span>
          </div>
          <div className="color-gray-medium">
            Get a quick overview on the issues in this session.
          </div>
        </div>
      }
      distance={30}
      theme={'light'}
      open={true}
      arrow={true}
    >
      {children}
    </Tooltip>
  );
}
